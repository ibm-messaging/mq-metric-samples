package main

/*
  Copyright (c) IBM Corporation 2016, 2018

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
	"os"
	"time"

	"github.com/ibm-messaging/mq-golang/mqmetric"
)

type mqExporterConfig struct {
	qMgrName              string
	replyQ                string
	monitoredQueues       string
	monitoredQueuesFile   string
	monitoredChannels     string
	monitoredChannelsFile string
	metaPrefix            string
	pollInterval          string
	pollIntervalDuration  time.Duration
	qStatus               bool

	cc mqmetric.ConnectionConfig

	httpListenPort string
	httpMetricPath string
	logLevel       string
	namespace      string

	locale string
}

const (
	defaultPort         = "9157" // Reserved in the prometheus wiki for MQ
	defaultNamespace    = "ibmmq"
	defaultPollInterval = "0s" // CHSTATUS is done every time there's a pull from Prometheus
)

var config mqExporterConfig

/*
initConfig parses the command line parameters. Note that the logging
package requires flag.Parse to be called before we can do things like
info/error logging

The default IP port for this monitor is registered with prometheus so
does not have to be provided.
*/
func initConfig() error {
	var err error

	flag.StringVar(&config.qMgrName, "ibmmq.queueManager", "", "Queue Manager name")
	flag.StringVar(&config.replyQ, "ibmmq.replyQueue", "SYSTEM.DEFAULT.MODEL.QUEUE", "Model Queue used for reply queues to collect data")

	flag.StringVar(&config.monitoredQueues, "ibmmq.monitoredQueues", "", "Patterns of queues to monitor")
	flag.StringVar(&config.monitoredQueuesFile, "ibmmq.monitoredQueuesFile", "", "File with patterns of queues to monitor")
	flag.StringVar(&config.metaPrefix, "metaPrefix", "", "Override path to monitoring resource topic")

	flag.StringVar(&config.monitoredChannels, "ibmmq.monitoredChannels", "", "Patterns of channels to monitor")
	flag.StringVar(&config.monitoredChannelsFile, "ibmmq.monitoredChannelsFile", "", "File with patterns of channels to monitor")

	flag.BoolVar(&config.cc.ClientMode, "ibmmq.client", false, "Connect as MQ client")

	flag.StringVar(&config.cc.UserId, "ibmmq.userid", "", "UserId for MQ connection")
	// If password is not given on command line (and it shouldn't be) then there's a prompt for stdin
	flag.StringVar(&config.cc.Password, "ibmmq.password", "", "Password for MQ connection")

	flag.StringVar(&config.httpListenPort, "ibmmq.httpListenPort", defaultPort, "HTTP Listener")
	flag.StringVar(&config.httpMetricPath, "ibmmq.httpMetricPath", "/metrics", "Path to exporter metrics")

	flag.StringVar(&config.logLevel, "log.level", "error", "Log level - debug, info, error")
	flag.StringVar(&config.namespace, "namespace", defaultNamespace, "Namespace for metrics")

	flag.StringVar(&config.pollInterval, "pollInterval", defaultPollInterval, "Frequency of checking channel status")

	// The locale ought to be discoverable from the environment, but making it an explicit config
	// parameter for now to aid testing, to override, and to ensure it's given in the MQ-known format
	// such as "Fr_FR"
	flag.StringVar(&config.locale, "locale", "", "Locale for translated metric descriptions")

	flag.BoolVar(&config.qStatus, "ibmmq.qStatus", false, "Add queue status polling")
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
