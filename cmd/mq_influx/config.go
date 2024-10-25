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
	"strings"

	cf "github.com/ibm-messaging/mq-metric-samples/v5/pkg/config"

	log "github.com/sirupsen/logrus"
)

type ConfigYInflux struct {
	BucketName      string `yaml:"bucketName"`
	DatabaseName    string `yaml:"databaseName"`
	DatabaseAddress string `yaml:"databaseAddress"`
	Userid          string `yaml:"databaseUser"`
	Org             string `yaml:"org"`
	Password        string `yaml:"databasePassword"`
	PasswordFile    string `yaml:"databasePasswordFile"`
	ApiToken        string `yaml:"apiToken"`
	ApiTokenFile    string `yaml:"apiTokenFile"`

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
	Filters    cf.ConfigYFilters
	Influx     ConfigYInflux `yaml:"influx"`
}

var config mqInfluxConfig
var cfy mqExporterConfigYaml

/*
initConfig parses the command line parameters.
*/
func initConfig() error {

	cf.InitConfig(&config.cf)
	cf.AddParm(&config.ci.BucketName, "", cf.CP_STR, "ibmmq.bucketName", "influx", "bucketName", "Name of database bucket")
	//cf.AddParm(&config.ci.DatabaseName, "", cf.CP_STR, "ibmmq.databaseName", "influx", "databaseName", "Name of database")
	cf.AddParm(&config.ci.DatabaseAddress, "", cf.CP_STR, "ibmmq.databaseAddress", "influx", "databaseAddress", "Address of database eg http://example.com:8086")
	cf.AddParm(&config.ci.Userid, "", cf.CP_STR, "ibmmq.databaseUserID", "influx", "databaseUser", "UserID to access the database")
	cf.AddParm(&config.ci.Interval, "10s", cf.CP_STR, "ibmmq.interval", "influx", "interval", "How long between each collection")
	cf.AddParm(&config.ci.Org, "", cf.CP_STR, "ibmmq.org", "influx", "org", "Organisation")
	cf.AddParm(&config.ci.PasswordFile, "", cf.CP_STR, "ibmmq.pwFile", "influx", "databasePasswordFile", "Where is password to database held temporarily")
	cf.AddParm(&config.ci.MaxErrors, 100, cf.CP_INT, "ibmmq.maxErrors", "influx", "maxErrors", "Maximum number of errors communicating with server before considered fatal")

	cf.AddParm(&config.ci.ApiTokenFile, "", cf.CP_STR, "ibmmq.apiTokenFile", "influx", "apiTokenFile", "Where is API Token for database access held temporarily")
	cf.AddParm(&config.ci.ApiToken, "", cf.CP_STR, "ibmmq.apiToken", "influx", "apiToken", "Where is API Token")

	err := cf.ParseParms()

	if err == nil {
		if config.cf.ConfigFile != "" {
			err = cf.ReadConfigFile(config.cf.ConfigFile, &cfy)
			if err == nil {
				cf.CopyYamlConfig(&config.cf, cfy.Global, cfy.Connection, cfy.Objects, cfy.Filters)
				config.ci.BucketName = cf.CopyParmIfNotSetStr("influx", "bucketName", cfy.Influx.BucketName)
				//config.ci.DatabaseName = cf.CopyParmIfNotSetStr("influx", "databaseName", cfy.Influx.DatabaseName)
				config.ci.DatabaseAddress = cf.CopyParmIfNotSetStr("influx", "databaseAddress", cfy.Influx.DatabaseAddress)
				config.ci.Userid = cf.CopyParmIfNotSetStr("influx", "databaseUser", cfy.Influx.Userid)
				config.ci.Interval = cf.CopyParmIfNotSetStr("influx", "interval", cfy.Influx.Interval)
				config.ci.Password = cf.CopyParmIfNotSetStr("influx", "databasePasswordFile", cfy.Influx.Password)
				config.ci.PasswordFile = cf.CopyParmIfNotSetStr("influx", "databasePasswordFile", cfy.Influx.PasswordFile)
				config.ci.MaxErrors = cf.CopyParmIfNotSetInt("influx", "maxErrors", cfy.Influx.MaxErrors)
				config.ci.ApiToken = cf.CopyParmIfNotSetStr("influx", "apiToken", cfy.Influx.ApiToken)
				config.ci.ApiTokenFile = cf.CopyParmIfNotSetStr("influx", "apiTokenFile", cfy.Influx.ApiTokenFile)
				config.ci.Org = cf.CopyParmIfNotSetStr("influx", "org", cfy.Influx.Org)
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
			if config.cf.PasswordFile == "" {
				config.cf.CC.Password = cf.GetPasswordFromStdin("Enter password for MQ: ")
			} else {
				config.cf.CC.Password, err = cf.GetPasswordFromFile(config.cf.PasswordFile, false)
			}
		}
	}

	// Process password for Influx connection.
	// Read password from a file if there is a userid on the command line.

	if err == nil {
		if config.ci.ApiToken == "" {
			if config.ci.ApiTokenFile != "" {
				config.ci.ApiToken, err = cf.GetPasswordFromFile(config.ci.ApiTokenFile, true)
			} else {
				if config.ci.Userid != "" && config.ci.Password == "" {
					config.ci.Userid = strings.TrimSpace(config.ci.Userid)
					config.ci.Password, err = cf.GetPasswordFromFile(config.ci.PasswordFile, true)
					if config.ci.Password == "" && err == nil {
						config.ci.Password = cf.GetPasswordFromStdin("Enter password for Influx: ")
					}
				}
			}
		}
	}

	if err == nil && config.cf.CC.UseResetQStats {
		log.Errorln("Warning: Data from 'RESET QSTATS' has been requested.")
		log.Errorln("Ensure no other monitoring applications are also using that command.")
	}
	return err
}
