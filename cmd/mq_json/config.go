package main

/*
  Copyright (c) IBM Corporation 2016

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
	"bufio"
	"flag"
	"fmt"
	"github.com/ibm-messaging/mq-golang/mqmetric"
	"os"
	"time"
)

type mqTTYConfig struct {
	qMgrName            string
	replyQ              string
	monitoredQueues     string
	monitoredQueuesFile string

	monitoredChannels     string
	monitoredChannelsFile string
	qStatus               bool

	metaPrefix           string
	pollInterval         string
	pollIntervalDuration time.Duration

	cc mqmetric.ConnectionConfig

	interval string

	logLevel string
}

var config mqTTYConfig

const (
	defaultPollInterval = "0s"
)

/*
initConfig parses the command line parameters.
*/
func initConfig() error {
	var err error

	flag.StringVar(&config.qMgrName, "ibmmq.queueManager", "", "Queue Manager name")
	flag.StringVar(&config.replyQ, "ibmmq.replyQueue", "SYSTEM.DEFAULT.MODEL.QUEUE", "Reply Queue to collect data")
	flag.StringVar(&config.monitoredQueues, "ibmmq.monitoredQueues", "", "Patterns of queues to monitor")
	flag.StringVar(&config.monitoredQueuesFile, "ibmmq.monitoredQueuesFile", "", "File with patterns of queues to monitor")

	flag.StringVar(&config.metaPrefix, "metaPrefix", "", "Override path to monitoring resource topic")

	flag.StringVar(&config.monitoredChannels, "ibmmq.monitoredChannels", "", "Patterns of channels to monitor")
	flag.StringVar(&config.monitoredChannelsFile, "ibmmq.monitoredChannelsFile", "", "File with patterns of channels to monitor")

	flag.StringVar(&config.interval, "ibmmq.interval", "10", "How many seconds between each collection")

	flag.BoolVar(&config.cc.ClientMode, "ibmmq.client", false, "Connect as MQ client")
	flag.BoolVar(&config.qStatus, "ibmmq.qstatus", false, "Add QSTATUS data")

	flag.StringVar(&config.logLevel, "log.level", "error", "Log level - debug, info, error")
	flag.StringVar(&config.pollInterval, "pollInterval", defaultPollInterval, "Frequency of checking channel status")

	flag.StringVar(&config.cc.UserId, "ibmmq.userid", "", "UserId for MQ connection")
	// If password is not given on command line (and it shouldn't be) then there's a prompt for stdin
	flag.StringVar(&config.cc.Password, "ibmmq.password", "", "Password for MQ connection")

	flag.Parse()

	if len(flag.Args()) > 0 {
		err = fmt.Errorf("Extra command line parameters given")
		flag.PrintDefaults()
	}

	if err == nil {
		if config.monitoredQueuesFile != "" {
			config.monitoredQueues, err = mqmetric.ReadPatterns(config.monitoredQueuesFile)
			if err != nil {
				err = fmt.Errorf("Failed to parse monitored queues file - %v", err)
			}
		}
	}

	if err == nil {
		if config.monitoredChannelsFile != "" {
			config.monitoredChannels, err = mqmetric.ReadPatterns(config.monitoredChannelsFile)
			if err != nil {
				err = fmt.Errorf("Failed to parse monitored channels file - %v", err)
			}
		}
	}

	if err == nil {
		if config.cc.UserId != "" && config.cc.Password == "" {
			// TODO: If stdin is a tty, then disable echo. Done differently on Windows and Unix
			scanner := bufio.NewScanner(os.Stdin)
			fmt.Printf("Enter password: \n")
			scanner.Scan()
			config.cc.Password = scanner.Text()
		}
	}

	if err == nil {
		config.pollIntervalDuration, err = time.ParseDuration(config.pollInterval)
		if err != nil {
			err = fmt.Errorf("Invalid value for interval parameter: %v", err)
		}
	}

	if err == nil {
		err = mqmetric.VerifyPatterns(config.monitoredQueues)
		if err != nil {
			err = fmt.Errorf("Invalid value for monitored queues: %v", err)
		}
	}

	if err == nil {
		err = mqmetric.VerifyPatterns(config.monitoredChannels)
		if err != nil {
			err = fmt.Errorf("Invalid value for monitored channels: %v", err)
		}
	}

	return err

}
