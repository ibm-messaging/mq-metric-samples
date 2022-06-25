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
	"fmt"
	cf "github.com/ibm-messaging/mq-metric-samples/v5/pkg/config"
	"time"
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
	cf.AddParm(&config.httpMetricPath, "/metrics", cf.CP_STR, "ibmmq.httpMetricPath", "prometheus", "metricsPath", "Path to exporter metrics")
	cf.AddParm(&config.keepRunning, true, cf.CP_BOOL, "ibmmq.keepRunning", "prometheus", "keepRunning", "Continue running after queue manager disconnection")
	cf.AddParm(&config.reconnectInterval, defaultReconnectInterval, cf.CP_STR, "ibmmq.reconnectInterval", "prometheus", "reconnectInterval", "How fast to validate connection attempts")
	cf.AddParm(&config.httpsCertFile, "", cf.CP_STR, "ibmmq.httpsCertFile", "prometheus", "httpsCertFile", "TLS public certificate file")
	cf.AddParm(&config.httpsKeyFile, "", cf.CP_STR, "ibmmq.httpsKeyFile", "prometheus", "httpsKeyFile", "TLS private key file")

	cf.AddParm(&config.namespace, defaultNamespace, cf.CP_STR, "namespace", "prometheus", "namespace", "Namespace for metrics")

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
		config.reconnectIntervalDuration, err = time.ParseDuration(config.reconnectInterval)
	}

	if err == nil {
		if config.cf.CC.UserId != "" && config.cf.CC.Password == "" {
			config.cf.CC.Password = cf.GetPasswordFromStdin("Enter password for MQ: ")
		}
	}

	if err == nil && config.cf.CC.UseResetQStats {
		fmt.Println("Warning: Data from 'RESET QSTATS' has been requested.")
		fmt.Println("Ensure no other monitoring applications are also using that command.")
	}

	return err
}
