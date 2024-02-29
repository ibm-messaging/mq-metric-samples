package main

/*
  Copyright (c) IBM Corporation 2024

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
	//"strings"

	cf "github.com/ibm-messaging/mq-metric-samples/v5/pkg/config"
	log "github.com/sirupsen/logrus"
)

type ConfigYOTel struct {
	Endpoint      string `yaml:"endpoint"`
	Insecure      string `yaml:"insecure" default:"false"`
	Interval      string `yaml:"interval"`
	MaxErrors     int    `yaml:"maxErrors"`
	LogLevel      string `yaml:"logLevel"`
	OverrideCType string `yaml:"overrideCType"`
}

type mqOTelConfig struct {
	cf            cf.Config
	ci            ConfigYOTel
	overrideCType bool
}

type mqExporterConfigYaml struct {
	Global     cf.ConfigYGlobal
	Connection cf.ConfigYConnection
	Objects    cf.ConfigYObjects
	Filters    cf.ConfigYFilters
	OTel       ConfigYOTel `yaml:"otel"`
}

var config mqOTelConfig
var cfy mqExporterConfigYaml

/*
initConfig parses the command line parameters.
*/
func initConfig() error {

	cf.InitConfig(&config.cf)

	cf.AddParm(&config.ci.Endpoint, "", cf.CP_STR, "ibmmq.otelEndpoint", "otel", "endpoint", "Where the OLTP data should be sent (empty for stdout)")
	cf.AddParm(&config.ci.Interval, "10s", cf.CP_STR, "ibmmq.interval", "otel", "interval", "How long between each collection")
	cf.AddParm(&config.ci.MaxErrors, 100, cf.CP_INT, "ibmmq.maxErrors", "otel", "maxErrors", "Maximum number of errors communicating with server before considered fatal")
	cf.AddParm(&config.ci.LogLevel, "", cf.CP_STR, "ibmmq.otelLogLevel", "otel", "logLevel", "For tracing OTel library packages")
	cf.AddParm(&config.ci.Insecure, "", cf.CP_STR, "ibmmq.otelInsecure", "otel", "insecure", "Connection to GRPC server used insecure protocol")
	cf.AddParm(&config.ci.OverrideCType, "", cf.CP_STR, "ibmmq.otelOverrideCType", "otel", "overrideCType", "Override data types for GRPC processing")

	err := cf.ParseParms()
	if err == nil {
		if config.cf.ConfigFile != "" {
			err = cf.ReadConfigFile(config.cf.ConfigFile, &cfy)
			if err == nil {
				cf.CopyYamlConfig(&config.cf, cfy.Global, cfy.Connection, cfy.Objects, cfy.Filters)
				config.ci.Endpoint = cf.CopyParmIfNotSetStr("otel", "endpoint", cfy.OTel.Endpoint)
				config.ci.Interval = cf.CopyParmIfNotSetStr("otel", "interval", cfy.OTel.Interval)
				config.ci.MaxErrors = cf.CopyParmIfNotSetInt("otel", "maxErrors", cfy.OTel.MaxErrors)
				config.ci.LogLevel = cf.CopyParmIfNotSetStr("otel", "logLevel", cfy.OTel.LogLevel)
				config.ci.Insecure = cf.CopyParmIfNotSetStr("otel", "insecure", cfy.OTel.Insecure)
				config.ci.OverrideCType = cf.CopyParmIfNotSetStr("otel", "overrideCType", cfy.OTel.OverrideCType)

			}
		}
	}

	if err == nil {
		cf.InitLog(config.cf)
	}

	if err == nil {
		// The override value will turn all metrics into Gauges instead of a mixture of Gauge and Counter
		// when sending across GRPC.
		// This preserves a degree of compatibility with the mq_prometheus collector in this repo and any dashboards.
		if config.ci.Endpoint != "" {
			config.overrideCType = cf.AsBool(config.ci.OverrideCType, false)
		} else {
			config.overrideCType = false
		}
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
