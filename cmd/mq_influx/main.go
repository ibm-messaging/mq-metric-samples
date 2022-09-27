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

	"github.com/ibm-messaging/mq-golang/v5/ibmmq"
	"github.com/ibm-messaging/mq-golang/v5/mqmetric"
	cf "github.com/ibm-messaging/mq-metric-samples/v5/pkg/config"
	client "github.com/influxdata/influxdb-client-go/v2"
	ilog "github.com/influxdata/influxdb-client-go/v2/log"

	log "github.com/sirupsen/logrus"
)

var BuildStamp string
var GitCommit string
var BuildPlatform string
var discoverConfig mqmetric.DiscoverConfig

func main() {
	var err error
	var c client.Client
	var d time.Duration

	cf.PrintInfo("IBM MQ metrics exporter for InfluxDB monitoring", BuildStamp, GitCommit, BuildPlatform)

	err = initConfig()

	if err == nil && config.cf.QMgrName == "" {
		log.Errorln("Must provide a queue manager name to connect to.")
		os.Exit(72)
	}
	if err == nil {
		d, err = time.ParseDuration(config.ci.Interval)
		if err != nil || d.Seconds() <= 1 {
			log.Errorln("Invalid or too short value for interval parameter: ", err)
			os.Exit(1)
		}

		// Connect and open standard queues
		err = mqmetric.InitConnection(config.cf.QMgrName, config.cf.ReplyQ, config.cf.ReplyQ2, &config.cf.CC)
	}
	if err == nil {
		log.Infoln("Connected to queue manager ", config.cf.QMgrName)
	} else {
		if mqe, ok := err.(mqmetric.MQMetricError); ok {
			mqrc := mqe.MQReturn.MQRC
			mqcc := mqe.MQReturn.MQCC
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

	// What metrics can the queue manager provide? Find out, and
	// subscribe.

	if err == nil {
		discoverConfig.MonitoredQueues.ObjectNames = config.cf.MonitoredQueues
		discoverConfig.MonitoredQueues.UseWildcard = true
		discoverConfig.MonitoredQueues.SubscriptionSelector = strings.ToUpper(config.cf.QueueSubscriptionSelector)
		discoverConfig.MetaPrefix = config.cf.MetaPrefix
		err = mqmetric.DiscoverAndSubscribe(discoverConfig)
		mqmetric.RediscoverAttributes(ibmmq.MQOT_CHANNEL, config.cf.MonitoredChannels)
		mqmetric.RediscoverAttributes(mqmetric.OT_CHANNEL_AMQP, config.cf.MonitoredAMQPChannels)

	}

	if err == nil {
		var compCode int32
		compCode, err = mqmetric.VerifyConfig()
		// We could choose to fail after a warning, but instead will continue for now
		if compCode == ibmmq.MQCC_WARNING {
			log.Println(err)
			err = nil
		}
	}

	if err == nil {
		mqmetric.ChannelInitAttributes()
		mqmetric.QueueInitAttributes()
		mqmetric.TopicInitAttributes()
		mqmetric.SubInitAttributes()
		mqmetric.QueueManagerInitAttributes()
		mqmetric.UsageInitAttributes()
		mqmetric.ClusterInitAttributes()
		mqmetric.ChannelAMQPInitAttributes()

	}

	// Go into main loop for sending data to database
	// Creating the client does not return an error; the error will
	// come during the write of the data.
	if err == nil {
		if config.ci.ApiToken == "" {
			config.ci.ApiToken = config.ci.Userid + ":" + config.ci.Password
		}
		c = client.NewClientWithOptions(config.ci.DatabaseAddress, config.ci.ApiToken,
			client.DefaultOptions().SetPrecision(time.Millisecond))
		defer c.Close()
		ilog.Log = nil
		for {
			Collect(c)
			time.Sleep(d)
		}

	}

	if err != nil {
		log.Fatal(err)
	}

	os.Exit(0)
}
