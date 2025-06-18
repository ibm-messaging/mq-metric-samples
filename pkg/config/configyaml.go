package config

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

// This package provides a set of common routines that can used by all the
// sample metric monitor programs to get the configuration from a YAML file.
// Settings in that file can be overridden on the command line or via environment variable
import (
	"fmt"
	"os"
	"strconv"

	"gopkg.in/yaml.v2"
)

type ConfigYGlobal struct {
	UseObjectStatus    string `yaml:"useObjectStatus" default:"true"`
	UseResetQStats     string `yaml:"useResetQStats" default:"false"`
	UsePublications    string `yaml:"usePublications" default:"true"`
	LogLevel           string `yaml:"logLevel"`
	MetaPrefix         string
	PollInterval       string `yaml:"pollInterval"`
	RediscoverInterval string `yaml:"rediscoverInterval"`
	TZOffset           string `yaml:"tzOffset"`
	Locale             string
}
type ConfigYConnection struct {
	QueueManager     string `yaml:"queueManager"`
	User             string
	Client           string `yaml:"clientConnection" default:"false"`
	Password         string
	PasswordFile     string            `yaml:"passwordFile"`
	ReplyQueue       string            `yaml:"replyQueue" `
	ReplyQueue2      string            `yaml:"replyQueue2"`
	DurableSubPrefix string            `yaml:"durableSubPrefix"`
	CcdtUrl          string            `yaml:"ccdtUrl"`
	ConnName         string            `yaml:"connName"`
	Channel          string            `yaml:"channel"`
	WaitInterval     string            `yaml:"waitInterval"`
	MetadataTags     []string          `yaml:"metadataTags"`
	MetadataValues   []string          `yaml:"metadataValues"`
	MetadataMap      map[string]string `yaml:"metadataMap"`
}
type ConfigYObjects struct {
	Queues       []string
	Channels     []string
	AMQPChannels []string `yaml:"amqpChannels"`
	MQTTChannels []string `yaml:"mqttChannels"`

	Topics        []string
	Subscriptions []string
	// These are left here for now so they can be recognised but will cause an error because the
	// real values have moved
	QueueSubscriptionSelector []string `yaml:"queueSubscriptionSelector"`
	ShowInactiveChannels      string   `yaml:"showInactiveChannels" default:"false"`
}

type ConfigYFilters struct {
	HideSvrConnJobname        string   `yaml:"hideSvrConnJobname" default:"false"`
	HideAMQPClientId          string   `yaml:"hideAMQPClientId" default:"false"`
	HideMQTTClientId          string   `yaml:"hideMQTTClientId" default:"false"`
	ShowInactiveChannels      string   `yaml:"showInactiveChannels" default:"false"`
	QueueSubscriptionSelector []string `yaml:"queueSubscriptionSelector"`
}

type ConfigMoved struct {
	QueueSubscriptionSelector string
	ShowInactiveChannels      string
}

var cfMoved ConfigMoved

func ReadConfigFile(f string, cmy interface{}) error {

	data, e2 := os.ReadFile(f)
	if e2 == nil {
		// fmt.Printf("Unparsed Data is\n %s\n", string(data))
		e2 = yaml.Unmarshal(data, cmy)
	}

	return e2
}

// The Go YAML parsing is not what you might expect for booleans - you are
// apparently unable to set a default of "true" for missing fields. So we read it
// as a string and parse that. The caller also sends in the default value if the string
// cannot be decoded.
func AsBool(s string, def bool) bool {
	b, err := strconv.ParseBool(s)
	if err == nil {
		//fmt.Printf("Value: %s Default: %v Returning: %v\n", s, def, b)
		return b
	} else {
		//fmt.Printf("Value: %s Default: %v Returning def: %v\n", s, def, def)
		return def
	}
}

func asInt(s string, def int) int {
	if s != "" {
		i, err := strconv.Atoi(s)
		if err == nil {
			return int(i)
		} else {
			return def
		}
	} else {
		return def
	}
}

// This handles the configuration parameters that are common to all the collectors. The individual
// collectors call similar code for their own specific attributes
func CopyYamlConfig(cm *Config, cyg ConfigYGlobal, cyc ConfigYConnection, cyo ConfigYObjects, cyf ConfigYFilters) {

	cm.CC.UseStatus = CopyParmIfNotSetBool("global", "useObjectStatus", AsBool(cyg.UseObjectStatus, true))
	cm.CC.UseResetQStats = CopyParmIfNotSetBool("global", "useResetQStats", AsBool(cyg.UseResetQStats, false))
	cm.CC.UsePublications = CopyParmIfNotSetBool("global", "usePublications", AsBool(cyg.UsePublications, true))

	cm.CC.ShowInactiveChannels = CopyParmIfNotSetBool("filters", "showInactiveChannels", AsBool(cyf.ShowInactiveChannels, false))
	cm.CC.HideSvrConnJobname = CopyParmIfNotSetBool("filters", "hideSvrConnJobname", AsBool(cyf.HideSvrConnJobname, false))
	cm.CC.HideAMQPClientId = CopyParmIfNotSetBool("filters", "hideAMQPClientId", AsBool(cyf.HideAMQPClientId, false))
	cm.CC.HideMQTTClientId = CopyParmIfNotSetBool("filters", "hideMQTTClientId", AsBool(cyf.HideMQTTClientId, false))

	cm.QueueSubscriptionSelector = CopyParmIfNotSetStrArray("filters", "queueSubscriptionSelector", cyf.QueueSubscriptionSelector)

	cm.LogLevel = CopyParmIfNotSetStr("global", "logLevel", cyg.LogLevel)
	cm.MetaPrefix = CopyParmIfNotSetStr("global", "metaprefix", cyg.MetaPrefix)
	cm.pollInterval = CopyParmIfNotSetStr("global", "pollInterval", cyg.PollInterval)
	cm.rediscoverInterval = CopyParmIfNotSetStr("global", "rediscoverInterval", cyg.RediscoverInterval)
	cm.TZOffsetString = CopyParmIfNotSetStr("global", "tzOffset", cyg.TZOffset)
	cm.Locale = CopyParmIfNotSetStr("global", "locale", cyg.Locale)

	cm.QMgrName = CopyParmIfNotSetStr("connection", "queueManager", cyc.QueueManager)
	cm.CC.CcdtUrl = CopyParmIfNotSetStr("connection", "ccdtUrl", cyc.CcdtUrl)
	cm.CC.ConnName = CopyParmIfNotSetStr("connection", "connName", cyc.ConnName)
	cm.CC.Channel = CopyParmIfNotSetStr("connection", "channel", cyc.Channel)
	cm.CC.ClientMode = CopyParmIfNotSetBool("connection", "clientConnection", AsBool(cyc.Client, false))
	cm.CC.UserId = CopyParmIfNotSetStr("connection", "user", cyc.User)
	cm.CC.Password = CopyParmIfNotSetStr("connection", "password", cyc.Password)
	cm.PasswordFile = CopyParmIfNotSetStr("connection", "passwordFile", cyc.PasswordFile)

	tmpInt := CopyParmIfNotSetStr("connection", "waitInterval", cyc.WaitInterval)
	cm.CC.WaitInterval = asInt(tmpInt, defaultWaitInterval)

	// This is one where we want to use the default non-null value if it's not been provided
	tmpQ := CopyParmIfNotSetStr("connection", "replyQueue", cyc.ReplyQueue)
	if tmpQ != "" {
		cm.ReplyQ = tmpQ
	}
	cm.ReplyQ2 = CopyParmIfNotSetStr("connection", "replyQueue2", cyc.ReplyQueue2)
	cm.CC.DurableSubPrefix = CopyParmIfNotSetStr("connection", "durableSubPrefix", cyc.DurableSubPrefix)

	cm.MonitoredQueues = CopyParmIfNotSetStrArray("objects", "queues", cyo.Queues)
	cm.MonitoredChannels = CopyParmIfNotSetStrArray("objects", "channels", cyo.Channels)
	cm.MonitoredAMQPChannels = CopyParmIfNotSetStrArray("objects", "amqpChannels", cyo.AMQPChannels)
	cm.MonitoredMQTTChannels = CopyParmIfNotSetStrArray("objects", "mqttChannels", cyo.MQTTChannels)

	cm.MonitoredTopics = CopyParmIfNotSetStrArray("objects", "topics", cyo.Topics)
	cm.MonitoredSubscriptions = CopyParmIfNotSetStrArray("objects", "subscriptions", cyo.Subscriptions)

	cfMoved.QueueSubscriptionSelector = CopyDeprecatedParmIfNotSetStrArray("objects", "queueSubscriptionSelector", cyo.QueueSubscriptionSelector)
	cfMoved.ShowInactiveChannels = CopyDeprecatedParmIfNotSetStr("objects", "showInactiveChannels", cyo.ShowInactiveChannels)

	// Prefer the YAML map construct instead of the array list. If the map is provided, then make sure
	// that the tags/values arrays are built only from the map. Note that there is no command-line or env var
	// equivalent of the map - those have to use the separate tags/values comma-separated strings.
	if cyc.MetadataMap == nil || len(cyc.MetadataMap) == 0 {
		cm.metadataTags = CopyParmIfNotSetStrArray("connection", "metadataTags", cyc.MetadataTags)
		cm.metadataValues = CopyParmIfNotSetStrArray("connection", "metadataValues", cyc.MetadataValues)
	} else {
		cm.metadataTags = ""
		cm.metadataValues = ""
		for k, v := range cyc.MetadataMap {
			cm.MetadataTagsArray = append(cm.MetadataTagsArray, k)
			cm.MetadataValuesArray = append(cm.MetadataValuesArray, v)
		}
	}

	return
}

// If the parameter has already been set by env var or cli, then the value in the main config structure is returned. Otherwise
// the value passed as the "val" parameter - from the YAML version of the configuration elements - is returned
func CopyParmIfNotSetBool(section string, name string, val bool) bool {
	v, s := copyParmIfNotSet(section, name, false)
	if s {
		return *(v).(*bool)
	} else {
		return val
	}
}
func CopyDeprecatedParmIfNotSetBool(section string, name string, val bool) bool {
	v, s := copyParmIfNotSet(section, name, true)
	if s {
		return *(v).(*bool)
	} else {
		return val
	}
}

func CopyParmIfNotSetStr(section string, name string, val string) string {
	v, s := copyParmIfNotSet(section, name, false)
	if s {
		return *(v).(*string)
	} else {
		return val
	}
}

func CopyDeprecatedParmIfNotSetStr(section string, name string, val string) string {
	v, s := copyParmIfNotSet(section, name, true)
	if s {
		return *(v).(*string)
	} else {
		return val
	}
}

func CopyParmIfNotSetStrArray(section string, name string, val []string) string {
	v, s := copyParmIfNotSet(section, name, false)
	if s {
		return *(v).(*string)
	} else {
		// Convert YAML arrays into the single string expected by the mqmetric package
		s := ""
		for i := 0; i < len(val); i++ {
			if i == 0 {
				s = val[0]
			} else {
				s += "," + val[i]
			}
		}
		return s
	}
}

func CopyDeprecatedParmIfNotSetStrArray(section string, name string, val []string) string {
	v, s := copyParmIfNotSet(section, name, true)
	if s {
		return *(v).(*string)
	} else {
		// Convert YAML arrays into the single string expected by the mqmetric package
		s := ""
		for i := 0; i < len(val); i++ {
			if i == 0 {
				s = val[0]
			} else {
				s += "," + val[i]
			}
		}
		return s
	}
}

func CopyParmIfNotSetInt(section string, name string, val int) int {
	v, s := copyParmIfNotSet(section, name, false)
	//fmt.Printf("Name:%s Val:%d s:%v v:%v\n", name, val, s, v)

	if s {
		return *(v).(*int)
	} else {
		return val
	}
}

// Gets the value set in the YAML structure, but only if it has not previously been
// set by the user via CLI or environment variable
//
// Debug of this is handled by direct Printfs as it's run before the logger is configured
func copyParmIfNotSet(section string, name string, deprecated bool) (interface{}, bool) {
	k := envVarKey(section, name)
	if p, ok := configParms[k]; ok {
		if p.userSet {
			//fmt.Printf("Returning data from %v\n", p)
			return p.loc, true
		} else {
			//fmt.Printf("Key %s has not been set by user\n", k)
		}
	} else {
		// If this happens, it indicates a problem in one of the config.go files so we leave it in.
		if !deprecated {
			fmt.Printf("Key %s not found in parms map\n", k)
		}
	}
	return nil, false
}
