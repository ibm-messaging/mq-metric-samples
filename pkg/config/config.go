package config

/*
  Copyright (c) IBM Corporation 2016, 2023

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
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/ibm-messaging/mq-golang/v5/mqmetric"
	log "github.com/sirupsen/logrus"
)

// Configuration attributes shared by all the monitor sample programs
type Config struct {
	ConfigFile string

	QMgrName string
	ReplyQ   string
	ReplyQ2  string

	MetaPrefix string

	TZOffsetString string
	Locale         string

	MonitoredQueues            string
	MonitoredQueuesFile        string
	MonitoredChannels          string
	MonitoredChannelsFile      string
	MonitoredAMQPChannels      string
	MonitoredAMQPChannelsFile  string
	MonitoredTopics            string
	MonitoredTopicsFile        string
	MonitoredSubscriptions     string
	MonitoredSubscriptionsFile string
	QueueSubscriptionSelector  string

	LogLevel string

	// This is used for DISPLAY xxSTATUS commands, but not the collection of published resource stats
	pollInterval         string
	PollIntervalDuration time.Duration

	// How frequently should we redrive the list of known queues from the wildcards
	rediscoverInterval string
	RediscoverDuration time.Duration

	CC mqmetric.ConnectionConfig
}

type ConfigParm struct {
	loc          interface{}
	defaultValue interface{}
	parmType     int
	userSet      bool

	cliName    string
	envSection string // These also match the YAML elements
	envName    string

	usage string
}

const (
	defaultPollInterval       = "0s"
	defaultTZOffset           = "0h"
	defaultRediscoverInterval = "1h"
	defaultWaitInterval       = 3   // seconds
	defaultWaitIntervalStr    = "3" // seconds
)

const (
	CP_STR  = 0
	CP_INT  = 1
	CP_BOOL = 2
)

var configParms map[string]*ConfigParm
var keys []string

func envVarKey(section string, name string) string {
	return strings.ToUpper("IBMMQ_" + section + "_" + name)
}

func AddParm(loc interface{}, defaultValue interface{}, parmType int, cliName string, envSection string, envName string, usage string) {
	p := new(ConfigParm)
	p.loc = loc
	p.defaultValue = defaultValue
	p.cliName = cliName
	p.envSection = envSection
	p.envName = envName
	p.usage = usage
	p.parmType = parmType
	p.userSet = false

	configParms[envVarKey(p.envSection, p.envName)] = p

}

func InitConfig(cm *Config) {
	configParms = make(map[string]*ConfigParm)

	// Setup a slightly non-default error handler that gets called if there are problems parsing the command line parms
	flag.Usage = func() {

		o := flag.CommandLine.Output()
		fmt.Fprintf(o, "Usage of %s: \n", os.Args[0])
		flag.PrintDefaults()

		fmt.Fprintf(o, "\n\nValid environment variables for configuration are :\n")
		i := 0
		for _, k := range keys {
			if cp, ok := configParms[k]; ok {
				if strings.HasPrefix(cp.cliName, "removed") {
					continue
				}
			}
			fmt.Fprintf(o, "%-32s ", k)
			i++
			if i%5 == 0 {
				fmt.Fprintf(o, "\n")
			}
		}
		fmt.Fprintf(o, "\n")
	}

	// Setup the CLI flags and the equivalent Environment variable names. The env vars are named
	// after the YAML model, so they are slightly different than the CLI flag names.
	//
	// If the CLI flag is set, it overrides the env var. The env var in turn overrides the "default" hardcoded
	// values for a parameter.
	AddParm(&cm.LogLevel, "error", CP_STR, "log.level", "global", "logLevel", "Log level - debug, info, error")
	AddParm(&cm.QMgrName, "", CP_STR, "ibmmq.queueManager", "connection", "queueManager", "Queue Manager name")
	AddParm(&cm.CC.CcdtUrl, "", CP_STR, "ibmmq.ccdtUrl", "connection", "ccdtUrl", "Path to CCDT")
	AddParm(&cm.CC.ConnName, "", CP_STR, "ibmmq.connName", "connection", "connName", "Connection Name")
	AddParm(&cm.CC.Channel, "", CP_STR, "ibmmq.channel", "connection", "channel", "Channel Name")
	AddParm(&cm.ReplyQ, "SYSTEM.DEFAULT.MODEL.QUEUE", CP_STR, "ibmmq.replyQueue", "connection", "replyQueue", "Reply Queue to collect data")
	AddParm(&cm.ReplyQ2, "", CP_STR, "ibmmq.replyQueue2", "connection", "replyQueue2", "Reply Queue to collect other data ")
	AddParm(&cm.CC.DurableSubPrefix, "", CP_STR, "ibmmq.durableSubPrefix", "connection", "durableSubPrefix", "Collector identifier when using Durable Subscriptions")

	AddParm(&cm.CC.WaitInterval, defaultWaitInterval, CP_INT, "ibmmq.waitInterval", "connection", "waitInterval", "Maximum wait time for queue manager responses")

	AddParm(&cm.MetaPrefix, "", CP_STR, "metaPrefix", "global", "metaPrefix", "Override path to monitoring resource topic")

	// Note that there are non-empty defaults for Topics and Subscriptions
	AddParm(&cm.MonitoredQueues, "", CP_STR, "ibmmq.monitoredQueues", "objects", "queues", "Patterns of queues to monitor")
	AddParm(&cm.MonitoredChannels, "", CP_STR, "ibmmq.monitoredChannels", "objects", "channels", "Patterns of channels to monitor")
	AddParm(&cm.MonitoredAMQPChannels, "", CP_STR, "ibmmq.monitoredAMQPChannels", "objects", "amqpChannels", "Patterns of AMQP channels to monitor")

	AddParm(&cm.MonitoredQueuesFile, "", CP_STR, "ibmmq.monitoredQueuesFile", "objects", "queuesFile", "File with patterns of queues to monitor")
	AddParm(&cm.MonitoredChannelsFile, "", CP_STR, "ibmmq.monitoredChannelsFile", "objects", "channelsFile", "File with patterns of channels to monitor")
	AddParm(&cm.MonitoredAMQPChannelsFile, "", CP_STR, "ibmmq.monitoredAMQPChannelsFile", "objects", "amqpChannelsFile", "File with patterns of AMQP channels to monitor")

	AddParm(&cm.MonitoredTopics, "#", CP_STR, "ibmmq.monitoredTopics", "objects", "topics", "Patterns of topics to monitor")
	AddParm(&cm.MonitoredSubscriptions, "*", CP_STR, "ibmmq.monitoredSubscriptions", "objects", "subscriptions", "Patterns of subscriptions to monitor")
	AddParm(&cm.MonitoredTopicsFile, "", CP_STR, "ibmmq.monitoredTopicsFile", "objects", "topicsFile", "File with patterns of topics to monitor")
	AddParm(&cm.MonitoredSubscriptionsFile, "", CP_STR, "ibmmq.monitoredSubscriptionsFile", "objects", "subscriptionsFile", "File with patterns of subscriptions to monitor")
	AddParm(&cm.QueueSubscriptionSelector, "", CP_STR, "ibmmq.queueSubscriptionSelector", "filters", "queueSubscriptionSelector", "Resource topic selection for queues")
	AddParm(&cm.CC.ShowInactiveChannels, false, CP_BOOL, "ibmmq.showInactiveChannels", "filters", "showInactiveChannels", "Show inactive channels (not just stopped ones)")

	AddParm(&cm.CC.HideSvrConnJobname, false, CP_BOOL, "ibmmq.hideSvrConnJobname", "filters", "hideSvrConnJobname", "Don't create multiple instances of SVRCONN information")
	AddParm(&cm.CC.HideAMQPClientId, false, CP_BOOL, "ibmmq.hideAMQPClientId", "filters", "hideAMQPClientId", "Don't create multiple instances of ClientID information")

	// qStatus was the original flag but prefer to use useStatus as more meaningful for all object types
	AddParm(&cm.CC.UseStatus, false, CP_BOOL, "ibmmq.qStatus", "global", "useObjectStatus", "Add metrics from the QSTATUS fields")
	AddParm(&cm.CC.UseStatus, false, CP_BOOL, "ibmmq.useStatus", "global", "useObjectStatus", "Add metrics from all object STATUS fields")
	AddParm(&cm.CC.UsePublications, true, CP_BOOL, "ibmmq.usePublications", "global", "usePublications", "Use resource publications. Set to false to monitor older Distributed platforms")
	AddParm(&cm.CC.UseResetQStats, false, CP_BOOL, "ibmmq.resetQStats", "global", "useResetQStats", "Use RESET QSTATS on z/OS queue managers")

	AddParm(&cm.CC.UserId, "", CP_STR, "ibmmq.userid", "connection", "user", "UserId for MQ connection")
	// If password is not given on command line (and it shouldn't be) then there's a prompt for stdin
	AddParm(&cm.CC.Password, "", CP_STR, "ibmmq.password", "connection", "password", "Password for MQ connection")
	AddParm(&cm.CC.ClientMode, false, CP_BOOL, "ibmmq.client", "connection", "clientConnection", "Connect as MQ client")

	AddParm(&cm.TZOffsetString, defaultTZOffset, CP_STR, "ibmmq.tzOffset", "global", "tzOffset", "Time difference between collector and queue manager")
	AddParm(&cm.pollInterval, defaultPollInterval, CP_STR, "pollInterval", "global", "pollInterval", "Frequency of issuing object status checks")
	AddParm(&cm.rediscoverInterval, defaultRediscoverInterval, CP_STR, "rediscoverInterval", "global", "rediscoverInterval", "Frequency of expanding wildcards for monitored queues")
	// The locale ought to be discoverable from the environment, but making it an explicit config
	// parameter for now to aid testing, to override, and to ensure it's given in the MQ-known format
	// such as "Fr_FR"
	AddParm(&cm.Locale, "", CP_STR, "locale", "global", "locale", "Locale for translated metric descriptions")

	// A YAML configuration file can be used instead of all the preceding parameters
	AddParm(&cm.ConfigFile, "", CP_STR, "f", "global", "configurationFile", "Configuration file")

	AddParm(&cfMoved.QueueSubscriptionSelector, "", CP_STR, "removed.queueSubscriptionSelector", "objects", "queueSubscriptionSelector", "Moved to FILTERS section")
	AddParm(&cfMoved.ShowInactiveChannels, "", CP_STR, "removed.showInactiveChannels", "objects", "showInactiveChannels", "Moved to FILTERS section")

}

func ParseParms() error {
	var err error

	// While there's no real reason to sort the config parameters, it makes it
	// a bit easier to see things during debug
	keys = make([]string, len(configParms))
	i := 0
	for k, _ := range configParms {
		keys[i] = k
		i++
	}
	sort.Strings(keys)

	// Now setup the command line flags. If an environment variable
	// is defined for a value, then make that the new "default" value. A CLI
	// setting will override that. But a YAML config setting will not.
	for _, envVarName := range keys {
		p := configParms[envVarName]
		envValue := os.Getenv(envVarName)
		//fmt.Printf("Env var for %s is '%s' (string)\n", envVarName, envValue)
		if envValue != "" {
			p.userSet = true
		}

		switch p.parmType {
		case CP_STR:
			if envValue != "" {
				p.defaultValue = envValue
			}
			flag.StringVar((p.loc).(*string), p.cliName, (p.defaultValue).(string), p.usage)
		case CP_INT:
			if envValue != "" {
				d, _ := strconv.ParseInt(envValue, 10, 0)
				p.defaultValue = int(d)
			}
			flag.IntVar((p.loc).(*int), p.cliName, (p.defaultValue).(int), p.usage)
		case CP_BOOL:
			if envValue != "" {
				p.defaultValue, _ = strconv.ParseBool(envValue)
			}
			flag.BoolVar((p.loc).(*bool), p.cliName, (p.defaultValue).(bool), p.usage)
		}

	}

	flag.Parse()

	// Now that the command line has been parsed, we will record which flags
	// were explicitly set there.

	for _, envVarName := range keys {
		p := configParms[envVarName]
		n := p.cliName
		found := cliSet(n)
		if found {
			//fmt.Printf("Flag %s was set on cmd line\n", n)
			p.userSet = true
		} else {
			//fmt.Printf("Flag %s was NOT set on cmd line\n", n)
		}
	}

	if len(flag.Args()) > 0 {
		err = fmt.Errorf("Unexpected additional command line parameters given.")
		fmt.Fprintf(flag.CommandLine.Output(), "%v\n\n", err)
		flag.Usage()
	}

	return err
}

// Was a flag explicitly set on the command line
func cliSet(n string) bool {
	found := false
	flag.Visit(func(f *flag.Flag) {
		if f.Name == n {
			found = true
		}
	})
	return found
}

func VerifyConfig(cm *Config, fullCf interface{}) error {
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
		if cm.MonitoredAMQPChannelsFile != "" {
			cm.MonitoredAMQPChannels, err = mqmetric.ReadPatterns(cm.MonitoredAMQPChannelsFile)
			if err != nil {
				err = fmt.Errorf("Failed to parse monitored AMQP channels file - %v", err)
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

	if err == nil {
		if cfMoved.QueueSubscriptionSelector != "" {
			err = fmt.Errorf("QueueSubscriptionSelector has moved to filters section of configuration")
		}
	}

	if err == nil {
		if cfMoved.ShowInactiveChannels != "" {
			err = fmt.Errorf("ShowInactiveChannels has moved to filters section of configuration")
		}
	}

	log.Debugf("VerifyConfig Config: %+v", fullCf)
	if err != nil {
		log.Debugf("VerifyConfig Error : %+v", err)
	}

	// log.Fatalf("Exiting immediately") // Used this temporarily to save time

	return err
}

func PrintInfo(title string, stamp string, commit string, buildPlatform string) {
	fmt.Fprintln(os.Stderr, title)
	if stamp != "" {
		fmt.Fprintf(os.Stderr, "Build         : %s\n", stamp)
	}
	if commit != "" {
		fmt.Fprintf(os.Stderr, "Commit Level  : %s\n", commit)
	}
	if buildPlatform != "" {
		fmt.Fprintf(os.Stderr, "Build Platform: %s\n", buildPlatform)
	}
	fmt.Fprintf(os.Stderr, "MQ Go Version : %s\n", MqGolangVersion)
	fmt.Fprintf(os.Stderr, "\n")
}

func InitLog(cm Config) {
	level, err := log.ParseLevel(cm.LogLevel)
	if err != nil {
		level = log.InfoLevel
	}
	log.SetLevel(level)
	logger := new(mqmetric.Logger)
	logger.Trace = log.Tracef
	logger.Debug = log.Debugf
	logger.Info = log.Infof
	logger.Warn = log.Warnf
	logger.Error = log.Errorf
	logger.Fatal = log.Fatalf
	logger.Panic = log.Panicf
	mqmetric.SetLogger(logger)
}
