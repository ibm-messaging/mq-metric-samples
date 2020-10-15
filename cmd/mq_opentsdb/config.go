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
	"flag"
	"fmt"

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
	OpenTSDB   ConfigYOpenTSDB `yaml:"opentsdb"`
}

var config mqOpenTSDBConfig
var cfy mqExporterConfigYaml

/*
initConfig parses the command line parameters.
*/
func initConfig() {
	var err error

	cf.InitConfig(&config.cf)

	flag.StringVar(&config.ci.DatabaseAddress, "ibmmq.databaseAddress", "", "Address of database eg http://example.com:4242")
	flag.StringVar(&config.ci.Interval, "ibmmq.interval", "10s", "How long between each collection")
	flag.IntVar(&config.ci.MaxErrors, "ibmmq.maxErrors", 100, "Maximum number of errors communicating with server before considered fatal")
	flag.IntVar(&config.ci.MaxPoints, "ibmmq.maxPoints", 30, "Maximum number of points to include in each write to the server")
	flag.StringVar(&config.ci.MetricPrefix, "ibmmq.seriesPrefix", "ibmmq", "Prefix for all the MQ metric series")

	flag.Parse()

	if len(flag.Args()) > 0 {
		err = fmt.Errorf("Extra command line parameters given")
		flag.PrintDefaults()
	}

	if err == nil {
		if config.cf.ConfigFile != "" {
			// Set defaults
			cfy.Global.UsePublications = true
			err = cf.ReadConfigFile(config.cf.ConfigFile, &cfy)
			if err == nil {
				cf.CopyYamlConfig(&config.cf, cfy.Global, cfy.Connection, cfy.Objects)
				config.ci = cfy.OpenTSDB
				if config.ci.MetricPrefix == "" {
					config.ci.MetricPrefix = "ibmmq"
				}
			}
		}
	}
	if err == nil {
		cf.InitLog(config.cf)
	}

	// Note that printing of the config information happens before any password
	// is read from a file.
	if err == nil {
		err = cf.VerifyConfig(&config.cf)
		log.Debugf("OpenTSDB config: +%v", &config.ci)
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
}
