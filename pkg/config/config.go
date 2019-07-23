package config

/*
  Copyright (c) IBM Corporation 2016, 2019

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

// This package provides a set of common routines that can used by all the
// sample metric monitor programs

import (
	"flag"
	"fmt"
	"github.com/ibm-messaging/mq-golang/mqmetric"
	log "github.com/sirupsen/logrus"
	"time"
)

// Configuration attributes shared by all the monitor sample programs
type Config struct {
	QMgrName string
	ReplyQ   string

	MetaPrefix string

	TZOffsetString string

	MonitoredQueues            string
	MonitoredQueuesFile        string
	MonitoredChannels          string
	MonitoredChannelsFile      string
	MonitoredTopics            string
	MonitoredTopicsFile        string
	MonitoredSubscriptions     string
	MonitoredSubscriptionsFile string

	LogLevel string

	// This is used for DISPLAY xxSTATUS commands, but not the collection of published resource stats
	pollInterval         string
	PollIntervalDuration time.Duration

	// How frequently should we redrive the list of known queues from the wildcards
	rediscoverInterval string
	RediscoverDuration time.Duration

	CC mqmetric.ConnectionConfig
}

const (
	defaultPollInterval       = "0s"
	defaultTZOffset           = "0h"
	defaultRediscoverInterval = "1h"
)

func InitConfig(cm *Config) {
	flag.StringVar(&cm.LogLevel, "log.level", "error", "Log level - debug, info, error")

	flag.StringVar(&cm.QMgrName, "ibmmq.queueManager", "", "Queue Manager name")
	flag.StringVar(&cm.ReplyQ, "ibmmq.replyQueue", "SYSTEM.DEFAULT.MODEL.QUEUE", "Reply Queue to collect data")

	flag.StringVar(&cm.MetaPrefix, "metaPrefix", "", "Override path to monitoring resource topic")

	// Note that there are non-empty defaults for Topics and Subscriptions
	flag.StringVar(&cm.MonitoredQueues, "ibmmq.monitoredQueues", "", "Patterns of queues to monitor")
	flag.StringVar(&cm.MonitoredQueuesFile, "ibmmq.monitoredQueuesFile", "", "File with patterns of queues to monitor")
	flag.StringVar(&cm.MonitoredChannels, "ibmmq.monitoredChannels", "", "Patterns of channels to monitor")
	flag.StringVar(&cm.MonitoredChannelsFile, "ibmmq.monitoredChannelsFile", "", "File with patterns of channels to monitor")
	flag.StringVar(&cm.MonitoredTopics, "ibmmq.monitoredTopics", "#", "Patterns of topics to monitor")
	flag.StringVar(&cm.MonitoredTopicsFile, "ibmmq.monitoredTopicsFile", "", "File with patterns of topics to monitor")
	flag.StringVar(&cm.MonitoredSubscriptions, "ibmmq.monitoredSubscriptions", "*", "Patterns of subscriptions to monitor")
	flag.StringVar(&cm.MonitoredSubscriptionsFile, "ibmmq.monitoredSubscriptionsFile", "", "File with patterns of subscriptions to monitor")

	// qStatus was the original flag but prefer to use useStatus as more meaningful for all object types
	flag.BoolVar(&cm.CC.UseStatus, "ibmmq.qStatus", false, "Add metrics from the QSTATUS fields")
	flag.BoolVar(&cm.CC.UseStatus, "ibmmq.useStatus", false, "Add metrics from all object STATUS fields")
	flag.BoolVar(&cm.CC.UsePublications, "ibmmq.usePublications", true, "Use resource publications. Set to false to monitor older Distributed platforms")
	flag.BoolVar(&cm.CC.UseResetQStats, "ibmmq.resetQStatus", false, "Use RESET QSTATS on z/OS queue managers")

	flag.StringVar(&cm.CC.UserId, "ibmmq.userid", "", "UserId for MQ connection")
	// If password is not given on command line (and it shouldn't be) then there's a prompt for stdin
	flag.StringVar(&cm.CC.Password, "ibmmq.password", "", "Password for MQ connection")
	flag.BoolVar(&cm.CC.ClientMode, "ibmmq.client", false, "Connect as MQ client")

	flag.StringVar(&cm.TZOffsetString, "ibmmq.tzOffset", defaultTZOffset, "Time difference between collector and queue manager")
	flag.StringVar(&cm.pollInterval, "pollInterval", defaultPollInterval, "Frequency of issuing object status checks")
	flag.StringVar(&cm.rediscoverInterval, "rediscoverInterval", defaultRediscoverInterval, "Frequency of expanding wildcards for monitored queues")

}

func VerifyConfig(cm *Config) error {
	var err error

	// If someone has explicitly said not to use publications, then they
	// must require use of the xxSTATUS commands. So override that flag even if they
	// have set UseStatus to false on the command line.
	if err == nil {
		if !cm.CC.UsePublications {
			cm.CC.UseStatus = true
		}
	}

	// RESET QSTATS does not strictly require use of the status commands
	// but it is based on the same cycle so force that option here
	if err == nil {
		if cm.CC.UseResetQStats {
			cm.CC.UseStatus = true
		}
	}

	if err == nil {
		if cm.MonitoredQueuesFile != "" {
			cm.MonitoredQueues, err = mqmetric.ReadPatterns(cm.MonitoredQueuesFile)
			if err != nil {
				err = fmt.Errorf("Failed to parse monitored queues file - %v", err)
			}
		}
	}

	if err == nil {
		if cm.MonitoredChannelsFile != "" {
			cm.MonitoredChannels, err = mqmetric.ReadPatterns(cm.MonitoredChannelsFile)
			if err != nil {
				err = fmt.Errorf("Failed to parse monitored channels file - %v", err)
			}
		}
	}

	if err == nil {
		if cm.MonitoredTopicsFile != "" {
			cm.MonitoredTopics, err = mqmetric.ReadPatterns(cm.MonitoredTopicsFile)
			if err != nil {
				err = fmt.Errorf("Failed to parse monitored topics file - %v", err)
			}
		}
	}

	if err == nil {
		if cm.MonitoredSubscriptionsFile != "" {
			cm.MonitoredSubscriptions, err = mqmetric.ReadPatterns(cm.MonitoredSubscriptionsFile)
			if err != nil {
				err = fmt.Errorf("Failed to parse monitored subscriptions file - %v", err)
			}
		}
	}

	if err == nil {
		err = mqmetric.VerifyQueuePatterns(cm.MonitoredQueues)
		if err != nil {
			err = fmt.Errorf("Invalid value for monitored queues: %v", err)
		}
	}

	if err == nil {
		err = mqmetric.VerifyPatterns(cm.MonitoredChannels)
		if err != nil {
			err = fmt.Errorf("Invalid value for monitored channels: %v", err)
		}
	}

	// Do not use VerifyPatterns for monitoredTopics or Subs as they follow a very different style
	if err == nil {
		offset, err := time.ParseDuration(cm.TZOffsetString)
		if err != nil {
			err = fmt.Errorf("Invalid value for time offset parameter: %v", err)
		} else {
			cm.CC.TZOffsetSecs = offset.Seconds()
		}
	}

	if err == nil {
		cm.PollIntervalDuration, err = time.ParseDuration(cm.pollInterval)
		if err != nil {
			err = fmt.Errorf("Invalid value for poll interval parameter: %v", err)
		}
	}

	if err == nil {
		cm.RediscoverDuration, err = time.ParseDuration(cm.rediscoverInterval)
		if err != nil {
			err = fmt.Errorf("Invalid value for rediscovery interval parameter: %v", err)
		}
	}

	return err
}

func PrintInfo(title string, stamp string, commit string, buildPlatform string) {
	fmt.Println(title)
	if stamp != "" {
		fmt.Println("Build         : " + stamp)
	}
	if commit != "" {
		fmt.Println("Commit Level  : " + commit)
	}
	if buildPlatform != "" {
		fmt.Println("Build Platform: " + buildPlatform)
	}
	fmt.Println("")
}

func InitLog(cm Config) {
	level, err := log.ParseLevel(cm.LogLevel)
	if err != nil {
		level = log.InfoLevel
	}
	log.SetLevel(level)
	logger := new(mqmetric.Logger)
	logger.Debug = log.Debugf
	logger.Info = log.Infof
	logger.Error = log.Errorf
	mqmetric.SetLogger(logger)
}
