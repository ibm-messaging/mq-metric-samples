package main

/*
  Copyright (c) IBM Corporation 2016,2020

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
	cf "github.com/ibm-messaging/mq-metric-samples/v5/pkg/config"
	"os"
)

type mqTTYConfig struct {
	cf cf.Config

	hostname  string
	hostlabel string // Used in the output string
	interval  string
}

type ConfigYColl struct {
	Interval string
	Hostname string `yaml:"hostname"`
}

type mqExporterConfigYaml struct {
	Global     cf.ConfigYGlobal
	Connection cf.ConfigYConnection
	Objects    cf.ConfigYObjects
	Collectd   ConfigYColl `yaml:"collectd"`
}

var config mqTTYConfig
var cfy mqExporterConfigYaml

/*
initConfig parses the command line parameters.
*/
func initConfig() error {

	cf.InitConfig(&config.cf)

	cf.AddParm(&config.interval, "60s", cf.CP_STR, "ibmmq.interval", "collectd", "interval", "How long between each collection")
	cf.AddParm(&config.hostname, "localhost", cf.CP_STR, "ibmmq.hostname", "collectd", "hostname", "Host to connect to")

	err := cf.ParseParms()

	if err == nil {
		if config.cf.ConfigFile != "" {
			err = cf.ReadConfigFile(config.cf.ConfigFile, &cfy)
			if err == nil {
				cf.CopyYamlConfig(&config.cf, cfy.Global, cfy.Connection, cfy.Objects)
				config.interval = cf.CopyParmIfNotSetStr("collectd", "interval", cfy.Collectd.Interval)
				config.hostname = cf.CopyParmIfNotSetStr("collectd", "hostname", cfy.Collectd.Hostname)
			}
		}
	}

	if err == nil {
		cf.InitLog(config.cf)
	}

	if err == nil {
		err = cf.VerifyConfig(&config.cf, config)
	}

	if err == nil {
		if config.cf.CC.UserId != "" && config.cf.CC.Password == "" {
			config.cf.CC.Password = cf.GetPasswordFromStdin("Enter password for MQ: ")
		}
	}

	// Don't want to use "localhost" as the tag in the metric printing
	if config.hostname == "localhost" || config.hostname == "" {
		config.hostlabel, _ = os.Hostname()
	} else {
		config.hostlabel = config.hostname
	}

	return err
}
