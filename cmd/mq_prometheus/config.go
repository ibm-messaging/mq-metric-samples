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
	"time"

	log "github.com/sirupsen/logrus"

	cf "github.com/ibm-messaging/mq-metric-samples/v5/pkg/config"
)

type mqExporterConfig struct {
	cf cf.Config // Common configuration attributes for all collectors

	httpListenPort            string
	httpListenHost            string
	httpMetricPath            string
	namespace                 string
	httpsCertFile             string
	httpsKeyFile              string
	keepRunning               bool
	reconnectIntervalDuration time.Duration
	reconnectInterval         string
	overrideCType             string
	overrideCTypeBool         bool
}

type ConfigYProm struct {
	Port              string
	Host              string
	MetricsPath       string `yaml:"metricsPath"`
	Namespace         string
	HttpsCertFile     string `yaml:"httpsCertFile"`
	HttpsKeyFile      string `yaml:"httpsKeyFile"`
	KeepRunning       bool   `yaml:"keepRunning"`
	ReconnectInterval string `yaml:"reconnectInterval"`
	OverrideCType     string `yaml:"overrideCType"`
}

type mqExporterConfigYaml struct {
	Global     cf.ConfigYGlobal
	Connection cf.ConfigYConnection
	Objects    cf.ConfigYObjects
	Filters    cf.ConfigYFilters
	Prometheus ConfigYProm
}

const (
	defaultPort              = "9157" // Reserved in the prometheus wiki for MQ
	defaultNamespace         = "ibmmq"
	defaultReconnectInterval = "5s"
	defaultMetricPath        = "/metrics"
)

var config mqExporterConfig
var cfy mqExporterConfigYaml

/*
initConfig parses the command line parameters. Note that the logging
package requires flag.Parse to be called before we can do things like
info/error logging

The default IP port for this monitor is registered with prometheus so
does not have to be provided.
*/
func initConfig() error {
	var err error

	cf.InitConfig(&config.cf)

	cf.AddParm(&config.httpListenPort, defaultPort, cf.CP_STR, "ibmmq.httpListenPort", "prometheus", "port", "HTTP(S) Listener Port")
	cf.AddParm(&config.httpListenHost, "", cf.CP_STR, "ibmmq.httpListenHost", "prometheus", "host", "HTTP(S) Listener Host")
	cf.AddParm(&config.httpMetricPath, defaultMetricPath, cf.CP_STR, "ibmmq.httpMetricPath", "prometheus", "metricsPath", "Path to exporter metrics")
	cf.AddParm(&config.keepRunning, true, cf.CP_BOOL, "ibmmq.keepRunning", "prometheus", "keepRunning", "Continue running after queue manager disconnection")
	cf.AddParm(&config.reconnectInterval, defaultReconnectInterval, cf.CP_STR, "ibmmq.reconnectInterval", "prometheus", "reconnectInterval", "How fast to validate connection attempts")
	cf.AddParm(&config.httpsCertFile, "", cf.CP_STR, "ibmmq.httpsCertFile", "prometheus", "httpsCertFile", "TLS public certificate file")
	cf.AddParm(&config.httpsKeyFile, "", cf.CP_STR, "ibmmq.httpsKeyFile", "prometheus", "httpsKeyFile", "TLS private key file")

	cf.AddParm(&config.namespace, defaultNamespace, cf.CP_STR, "namespace", "prometheus", "namespace", "Namespace for metrics")
	cf.AddParm(&config.overrideCType, "", cf.CP_STR, "ibmmq.otelOverrideCType", "prometheus", "overrideCType", "Override default data types to give mixture of Counters and Gauges")

	err = cf.ParseParms()

	if err == nil {
		if config.cf.ConfigFile != "" {

			err = cf.ReadConfigFile(config.cf.ConfigFile, &cfy)
			if err == nil {
				cf.CopyYamlConfig(&config.cf, cfy.Global, cfy.Connection, cfy.Objects, cfy.Filters)
				config.httpListenPort = cf.CopyParmIfNotSetStr("prometheus", "port", cfy.Prometheus.Port)
				config.httpListenHost = cf.CopyParmIfNotSetStr("prometheus", "host", cfy.Prometheus.Host)
				config.httpMetricPath = cf.CopyParmIfNotSetStr("prometheus", "MetricsPath", cfy.Prometheus.MetricsPath)
				config.namespace = cf.CopyParmIfNotSetStr("prometheus", "namespace", cfy.Prometheus.Namespace)

				config.httpsCertFile = cf.CopyParmIfNotSetStr("prometheus", "httpsCertFile", cfy.Prometheus.HttpsCertFile)
				config.httpsKeyFile = cf.CopyParmIfNotSetStr("prometheus", "httpsKeyFile", cfy.Prometheus.HttpsKeyFile)

				config.keepRunning = cf.CopyParmIfNotSetBool("prometheus", "keepRunning", cfy.Prometheus.KeepRunning)
				if cfy.Prometheus.ReconnectInterval == "" {
					cfy.Prometheus.ReconnectInterval = defaultReconnectInterval
				}
				config.reconnectInterval = cf.CopyParmIfNotSetStr("prometheus", "reconnectInterval", cfy.Prometheus.ReconnectInterval)
				config.overrideCType = cf.CopyParmIfNotSetStr("prometheus", "overrideCType", cfy.Prometheus.OverrideCType)

			}
		}
	}

	if err == nil {
		if config.httpMetricPath == "" {
			config.httpMetricPath = defaultMetricPath
		}
		if config.namespace == "" {
			config.namespace = defaultNamespace
		}
	}

	if err == nil {
		cf.InitLog(config.cf)
	}

	if err == nil {
		// This preserves a degree of compatibility for the mq_prometheus collector in this repo and any dashboards.
		config.overrideCTypeBool = cf.AsBool(config.overrideCType, false)

		// But we don't need to keep backwards compatibility for the Events Statistics collection model
		if config.cf.CC.UseStatistics && !config.overrideCTypeBool {
			log.Warn("Using Statistics Events: Forcing overrideCType to true")
			config.overrideCTypeBool = true
		}
	}

	if err == nil {
		err = cf.VerifyConfig(&config.cf, config)
	}

	if err == nil {
		if config.reconnectInterval == "" {
			config.reconnectInterval = defaultReconnectInterval
		}
		config.reconnectIntervalDuration, err = time.ParseDuration(config.reconnectInterval)
	}

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
		log.Warn("Warning: Data from 'RESET QSTATS' has been requested.")
		log.Warn("Ensure no other monitoring applications are also using that command.")
	}

	return err
}
