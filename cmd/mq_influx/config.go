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
	"flag"
	"fmt"
	"strings"

	cf "github.com/ibm-messaging/mq-metric-samples/v5/pkg/config"

	log "github.com/sirupsen/logrus"
)

type ConfigYInflux struct {
	DatabaseName    string `yaml:"databaseName"`
	DatabaseAddress string `yaml:"databaseAddress"`
	Userid          string `yaml:"databaseUser"`
	Password        string `yaml:"databasePassword"`
	PasswordFile    string `yaml:"databasePasswordFile"`

	Interval  string
	MaxErrors int `yaml:"maxErrors"`
}

type mqInfluxConfig struct {
	cf cf.Config
	ci ConfigYInflux
}

type mqExporterConfigYaml struct {
	Global     cf.ConfigYGlobal
	Connection cf.ConfigYConnection
	Objects    cf.ConfigYObjects
	Influx     ConfigYInflux `yaml:"influx"`
}

var config mqInfluxConfig
var cfy mqExporterConfigYaml

/*
initConfig parses the command line parameters.
*/
func initConfig() error {
	var err error

	cf.InitConfig(&config.cf)

	flag.StringVar(&config.ci.DatabaseName, "ibmmq.databaseName", "", "Name of database")
	flag.StringVar(&config.ci.DatabaseAddress, "ibmmq.databaseAddress", "", "Address of database eg http://example.com:8086")
	flag.StringVar(&config.ci.Userid, "ibmmq.databaseUserID", "", "UserID to access the database")
	flag.StringVar(&config.ci.Interval, "ibmmq.interval", "10s", "How long between each collection")
	flag.StringVar(&config.ci.PasswordFile, "ibmmq.pwFile", "", "Where is password to database held temporarily")
	flag.IntVar(&config.ci.MaxErrors, "ibmmq.maxErrors", 100, "Maximum number of errors communicating with server before considered fatal")

	flag.Parse()

	if len(flag.Args()) > 0 {
		err = fmt.Errorf("Extra command line parameters given")
		flag.PrintDefaults()
	}

	if err == nil {
		if config.cf.ConfigFile != "" {
			// Set defaults
			cfy.Global.UsePublications = true
			err := cf.ReadConfigFile(config.cf.ConfigFile, &cfy)
			if err == nil {
				cf.CopyYamlConfig(&config.cf, cfy.Global, cfy.Connection, cfy.Objects)
				config.ci = cfy.Influx

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
		log.Debugf("Influx config: +%v", &config.ci)
	}

	// Process password for MQ connection
	if err == nil {
		if config.cf.CC.UserId != "" && config.cf.CC.Password == "" {
			config.cf.CC.Password = cf.GetPasswordFromStdin("Enter password for MQ: ")
		}
	}

	// Process password for Influx connection.
	// Read password from a file if there is a userid on the command line
	// Delete the file after reading it.
	if err == nil {
		if config.ci.Userid != "" && config.ci.Password == "" {
			config.ci.Userid = strings.TrimSpace(config.ci.Userid)
			config.ci.Password, err = cf.GetPasswordFromFile(config.ci.PasswordFile, true)
			if config.ci.Password == "" && err == nil {
				config.ci.Password = cf.GetPasswordFromStdin("Enter password for Influx: ")
			}
		}
	}

	if err == nil && config.cf.CC.UseResetQStats {
		log.Errorln("Warning: Data from 'RESET QSTATS' has been requested.")
		log.Errorln("Ensure no other monitoring applications are also using that command.")
	}
	return err
}
