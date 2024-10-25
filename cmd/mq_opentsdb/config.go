package main

/*
  Copyright (c) IBM Corporation 2016,2021

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

type ConfigYOpenTSDB struct {
	// OpenTSDB does not currently have an authentication mechanism so no user/passwd fields needed
	DatabaseAddress string `yaml:"databaseAddress"`

	Interval     string
	MaxErrors    int    `yaml:"maxErrors"`
	MaxPoints    int    `yaml:"maxPoints"`
	MetricPrefix string `yaml:"seriesPrefix"`
}

type mqOpenTSDBConfig struct {
	cf cf.Config
	ci ConfigYOpenTSDB
}

type mqExporterConfigYaml struct {
	Global     cf.ConfigYGlobal
	Connection cf.ConfigYConnection
	Objects    cf.ConfigYObjects
	Filters    cf.ConfigYFilters
	OpenTSDB   ConfigYOpenTSDB `yaml:"opentsdb"`
}

var config mqOpenTSDBConfig
var cfy mqExporterConfigYaml

/*
initConfig parses the command line parameters.
*/
func initConfig() error {
	var err error

	cf.InitConfig(&config.cf)

	cf.AddParm(&config.ci.DatabaseAddress, "", cf.CP_STR, "ibmmq.databaseAddress", "opentsdb", "databaseAddress", "Address of database eg http://example.com:4242")
	cf.AddParm(&config.ci.Interval, "10s", cf.CP_STR, "ibmmq.interval", "opentsdb", "interval", "How long between each collection")
	cf.AddParm(&config.ci.MaxErrors, 100, cf.CP_INT, "ibmmq.maxErrors", "opentsdb", "maxerrors", "Maximum number of errors communicating with server before considered fatal")
	cf.AddParm(&config.ci.MaxPoints, 30, cf.CP_INT, "ibmmq.maxPoints", "opentsdb", "maxPoints", "Maximum number of points to include in each write to the server")
	cf.AddParm(&config.ci.MetricPrefix, "ibmmq", cf.CP_STR, "ibmmq.seriesPrefix", "opentsdb", "seriesPrefix", "Prefix for all the MQ metric series")

	err = cf.ParseParms()

	if err == nil {
		if config.cf.ConfigFile != "" {
			err = cf.ReadConfigFile(config.cf.ConfigFile, &cfy)
			if err == nil {
				cf.CopyYamlConfig(&config.cf, cfy.Global, cfy.Connection, cfy.Objects, cfy.Filters)
				config.ci.DatabaseAddress = cf.CopyParmIfNotSetStr("opentsdb", "databaseAddress", cfy.OpenTSDB.DatabaseAddress)
				config.ci.Interval = cf.CopyParmIfNotSetStr("opentsdb", "interval", cfy.OpenTSDB.Interval)
				config.ci.MaxErrors = cf.CopyParmIfNotSetInt("opentsdb", "maxErrors", cfy.OpenTSDB.MaxErrors)
				config.ci.MaxPoints = cf.CopyParmIfNotSetInt("opentsdb", "maxPoints", cfy.OpenTSDB.MaxPoints)
				config.ci.MetricPrefix = cf.CopyParmIfNotSetStr("opentsdb", "seriesPrefix", cfy.OpenTSDB.MetricPrefix)
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
		log.Debugf("OpenTSDB config: +%v", &config.ci)
	}

	// Process password for MQ connection
	if err == nil {
		if config.cf.CC.UserId != "" && config.cf.CC.Password == "" {
			if config.cf.PasswordFile == "" {
				config.cf.CC.Password = cf.GetPasswordFromStdin("Enter password for MQ: ")
			} else {
				config.cf.CC.Password, err = cf.GetPasswordFromFile(config.cf.PasswordFile, false)
			}
		}
	}

	if err == nil && config.cf.CC.UseResetQStats {
		log.Errorln("Warning: Data from 'RESET QSTATS' has been requested.")
		log.Errorln("Ensure no other monitoring applications are also using that command.")
	}

	return err
}
