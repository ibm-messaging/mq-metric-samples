package config

/*
  Copyright (c) IBM Corporation 2016, 2020

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
	"gopkg.in/yaml.v2"
	"io/ioutil"
)

type ConfigYGlobal struct {
	UseObjectStatus    bool   `yaml:"useObjectStatus" default:true`
	UseResetQStats     bool   `yaml:"useResetQStats" default:false`
	UsePublications    bool   `yaml:"usePublications" default:true`
	LogLevel           string `yaml:"logLevel"`
	MetaPrefix         string
	PollInterval       string `yaml:"pollInterval"`
	RediscoverInterval string `yaml:"rediscoverInterval"`
	TZOffset           string `yaml:"tzOffset"`
	Locale             string
}
type ConfigYConnection struct {
	QueueManager string `yaml:"queueManager"`
	User         string
	Client       bool `yaml:"clientConnection"`
	Password     string
	ReplyQueue   string `yaml:"replyQueue"`
	CcdtUrl      string `yaml:"ccdtUrl"`
	ConnName     string `yaml:"connName"`
	Channel      string `yaml:"channel"`
}
type ConfigYObjects struct {
	Queues                    []string
	QueueSubscriptionSelector []string `yaml:"queueSubscriptionSelector"`
	Channels                  []string
	Topics                    []string
	Subscriptions             []string
	ShowInactiveChannels      bool `yaml:"showInactiveChannels"`
}

func ReadConfigFile(f string, cmy interface{}) error {

	data, e2 := ioutil.ReadFile(f)
	if e2 == nil {
		e2 = yaml.Unmarshal(data, cmy)
	}

	return e2
}

func CopyYamlConfig(cm *Config, cyg ConfigYGlobal, cyc ConfigYConnection, cyo ConfigYObjects) {
	cm.CC.UseStatus = cyg.UseObjectStatus
	cm.CC.UseResetQStats = cyg.UseResetQStats
	cm.CC.UsePublications = cyg.UsePublications
	cm.CC.ShowInactiveChannels = cyo.ShowInactiveChannels

	cm.LogLevel = copyIfSet(cm.LogLevel, cyg.LogLevel)
	cm.MetaPrefix = copyIfSet(cm.MetaPrefix, cyg.MetaPrefix)
	cm.pollInterval = copyIfSet(cm.pollInterval, cyg.PollInterval)
	cm.rediscoverInterval = copyIfSet(cm.rediscoverInterval, cyg.RediscoverInterval)
	cm.TZOffsetString = copyIfSet(cm.TZOffsetString, cyg.TZOffset)
	cm.Locale = copyIfSet(cm.Locale, cyg.Locale)

	cm.QMgrName = copyIfSet(cm.QMgrName, cyc.QueueManager)
	cm.CC.CcdtUrl = copyIfSet(cm.CC.CcdtUrl, cyc.CcdtUrl)
	cm.CC.ConnName = copyIfSet(cm.CC.ConnName, cyc.ConnName)
	cm.CC.Channel = copyIfSet(cm.CC.Channel, cyc.Channel)
	cm.CC.ClientMode = cyc.Client
	cm.CC.UserId = copyIfSet(cm.CC.UserId, cyc.User)
	cm.CC.Password = copyIfSet(cm.CC.Password, cyc.Password)
	cm.ReplyQ = copyIfSet(cm.ReplyQ, cyc.ReplyQueue)

	cm.MonitoredQueues = copyIfSetArray(cm.MonitoredQueues, cyo.Queues)
	cm.MonitoredChannels = copyIfSetArray(cm.MonitoredChannels, cyo.Channels)
	cm.MonitoredTopics = copyIfSetArray(cm.MonitoredTopics, cyo.Topics)
	cm.MonitoredSubscriptions = copyIfSetArray(cm.MonitoredSubscriptions, cyo.Subscriptions)
	cm.QueueSubscriptionSelector = copyIfSetArray(cm.QueueSubscriptionSelector, cyo.QueueSubscriptionSelector)

	return
}

func copyIfSetArray(a string, b []string) string {
	s := a
	for i := 0; i < len(b); i++ {
		if i == 0 {
			s = b[0]
		} else {
			s += "," + b[i]
		}
	}
	return s
}

func copyIfSet(a string, b string) string {
	if b != "" {
		return b
	} else {
		return a
	}
}
