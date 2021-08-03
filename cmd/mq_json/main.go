package main

/*
  Copyright (c) IBM Corporation 2016, 2021

  Licensed under the Apache License, Version 2.0 (the "License");
  you may not use this file except in compliance with the License.
  You may obtain a copy of the License at

  http://www.apache.org/licenses/LICENSE-2.0

  Unless required by applicable law or agreed to in writing, software
  distributed under the License is distributed on an "AS IS" BASIS,
  WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
  See the License for the specific

   Contributors:
     Mark Taylor - Initial Contribution
*/

import (
	"os"
	"strings"
	"time"

	ibmmq "github.com/ibm-messaging/mq-golang/v5/ibmmq"
	mqmetric "github.com/ibm-messaging/mq-golang/v5/mqmetric"
	log "github.com/sirupsen/logrus"
)

var BuildStamp string
var GitCommit string
var BuildPlatform string
var discoverConfig mqmetric.DiscoverConfig

// Print this via the logger rather than direct to stdout so it can be
// avoided if someone is using the stdout stream as the JSON input to a parser
func printInfo(title string, stamp string, commit string, buildPlatform string) {
	log.Infoln(title)
	if stamp != "" {
		log.Infoln("Build         : " + stamp)
	}
	if commit != "" {
		log.Infoln("Commit Level  : " + commit)
	}
	if buildPlatform != "" {
		log.Infoln("Build Platform: " + buildPlatform)
	}
	log.Println("")
}

func main() {
	var err error
	var d time.Duration

	err = initConfig()

	printInfo("Starting IBM MQ metrics exporter for JSON", BuildStamp, GitCommit, BuildPlatform)

	if err == nil && config.cf.QMgrName == "" {
		log.Errorln("Must provide a queue manager name to connect to.")
		os.Exit(72)
	}
	if err == nil {
		d, err = time.ParseDuration(config.interval)
		if err != nil || d.Seconds() <= 1 {
			log.Errorln("Invalid or too short value for interval parameter: ", err)
			os.Exit(1)
		}

		if config.cf.CC.UseResetQStats {
			log.Warnln("Warning: Data from 'RESET QSTATS' has been requested. Ensure no other monitoring applications are also using that command.")
		}

		log.Infoln("Starting IBM MQ metrics exporter for JSON")

		// Connect and open standard queues
		err = mqmetric.InitConnection(config.cf.QMgrName, config.cf.ReplyQ, &config.cf.CC)
	}

	if err == nil {
		log.Infoln("Connected to queue manager ", config.cf.QMgrName)
	} else {
		if mqe, ok := err.(mqmetric.MQMetricError); ok {
			mqrc := mqe.MQReturn.MQRC
			mqcc := mqe.MQReturn.MQRC

			if mqrc == ibmmq.MQRC_STANDBY_Q_MGR {
				log.Errorln(err)
				os.Exit(30) // This is the same as the strmqm return code for "active instance running elsewhere"
			} else if mqcc == ibmmq.MQCC_WARNING {
				log.Infoln("Connected to queue manager ", config.cf.QMgrName)
				// Report the error but allow it to continue
				log.Errorln(err)
				err = nil
			}
		}
	}

	if err == nil {
		defer mqmetric.EndConnection()
	}

	if err == nil {
		mqmetric.ChannelInitAttributes()
		mqmetric.QueueInitAttributes()
		mqmetric.TopicInitAttributes()
		mqmetric.SubInitAttributes()
		mqmetric.QueueManagerInitAttributes()
		mqmetric.UsageInitAttributes()
		mqmetric.ClusterInitAttributes()
	}
	// What metrics can the queue manager provide? Find out, and
	// subscribe.
	if err == nil {
		// Do we need to expand wildcarded queue names
		// or use the wildcard as-is in the subscriptions
		wildcardResource := true
		if config.cf.MetaPrefix != "" {
			wildcardResource = false
		}
		discoverConfig.MonitoredQueues.ObjectNames = config.cf.MonitoredQueues
		discoverConfig.MonitoredQueues.UseWildcard = wildcardResource
		discoverConfig.MetaPrefix = config.cf.MetaPrefix
		discoverConfig.MonitoredQueues.SubscriptionSelector = strings.ToUpper(config.cf.QueueSubscriptionSelector)

		err = mqmetric.DiscoverAndSubscribe(discoverConfig)
		// Also get the static attributes for any configured channels
		mqmetric.RediscoverAttributes(ibmmq.MQOT_CHANNEL, config.cf.MonitoredChannels)
	}

	// Go into main loop for sending data to stdout
	// This program runs forever
	if err == nil {
		for {
			Collect()
			time.Sleep(d)
		}
	}

	if err != nil {
		log.Fatal(err)
	}

	os.Exit(0)
}
