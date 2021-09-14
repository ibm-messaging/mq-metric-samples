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
	cf "github.com/ibm-messaging/mq-metric-samples/v5/pkg/config"
	log "github.com/sirupsen/logrus"
)

type ConfigYCloudwatch struct {
	Region    string
	Namespace string

	Interval  string
	MaxErrors int `yaml:"maxErrors"`
	MaxPoints int `yaml:"maxPoints"`
}

type mqCloudWatchConfig struct {
	cf cf.Config
	ci ConfigYCloudwatch
}

type mqExporterConfigYaml struct {
	Global     cf.ConfigYGlobal
	Connection cf.ConfigYConnection
	Objects    cf.ConfigYObjects
	Cloudwatch ConfigYCloudwatch `yaml:"cloudwatch"`
}

var config mqCloudWatchConfig
var cfy mqExporterConfigYaml

/*
initConfig parses the command line parameters.
*/
func initConfig() error {

	var err error

	cf.InitConfig(&config.cf)

	cf.AddParm(&config.ci.Region, "", cf.CP_STR, "ibmmq.awsregion", "cloudwatch", "awsregion", "AWS Region to connect to")
	cf.AddParm(&config.ci.Namespace, "IBM/MQ", cf.CP_STR, "ibmmq.namespace", "cloudwatch", "namespace", "Namespace for metrics")

	cf.AddParm(&config.ci.Interval, "60s", cf.CP_STR, "ibmmq.interval", "cloudwatch", "interval", "How long between each collection")
	cf.AddParm(&config.ci.MaxErrors, 10000, cf.CP_INT, "ibmmq.maxErrors", "cloudwatch", "maxerrors", "Maximum number of errors communicating with server before considered fatal")
	cf.AddParm(&config.ci.MaxPoints, 20, cf.CP_INT, "ibmmq.maxPoints", "cloudwatch", "maxpoints", "Maximum number of points to include in each write to the server")

	err = cf.ParseParms()

	if err == nil {
		if config.cf.ConfigFile != "" {
			err = cf.ReadConfigFile(config.cf.ConfigFile, &cfy)
			if err == nil {
				cf.CopyYamlConfig(&config.cf, cfy.Global, cfy.Connection, cfy.Objects)
				config.ci.Region = cf.CopyParmIfNotSetStr("cloudwatch", "awsregion", cfy.Cloudwatch.Region)
				config.ci.Namespace = cf.CopyParmIfNotSetStr("cloudwatch", "namespace", cfy.Cloudwatch.Namespace)

				config.ci.Interval = cf.CopyParmIfNotSetStr("cloudwatch", "interval", cfy.Cloudwatch.Interval)
				config.ci.MaxErrors = cf.CopyParmIfNotSetInt("cloudwatch", "maxerrors", cfy.Cloudwatch.MaxErrors)
				config.ci.MaxPoints = cf.CopyParmIfNotSetInt("cloudwatch", "maxpoints", cfy.Cloudwatch.MaxPoints)

			}
		}
	}

	if err == nil {
		cf.InitLog(config.cf)
	}

	// Note that printing of the config information happens before any password
	// is read from a file.
	if err == nil {
		err = cf.VerifyConfig(&config.cf, config)
	}

	// Process password for MQ connection
	if err == nil {
		if config.cf.CC.UserId != "" && config.cf.CC.Password == "" {
			config.cf.CC.Password = cf.GetPasswordFromStdin("Enter password for MQ: ")
		}
	}

	if err == nil && config.cf.CC.UseResetQStats {
		log.Errorln("Warning: Data from 'RESET QSTATS' has been requested.")
		log.Errorln("Ensure no other monitoring applications are also using that command.")
	}

	return err
}
