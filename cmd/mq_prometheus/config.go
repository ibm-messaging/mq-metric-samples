package main

/*
  Copyright (c) IBM Corporation 2016, 2019

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
)

type mqExporterConfig struct {
	cf cf.Config // Common configuration attributes for all collectors

	httpListenPort string
	httpMetricPath string
	namespace      string
}

type ConfigYProm struct {
	Port        string
	MetricsPath string `yaml:"metricsPath"`
	Namespace   string
}
type mqExporterConfigYaml struct {
	Global     cf.ConfigYGlobal
	Connection cf.ConfigYConnection
	Objects    cf.ConfigYObjects
	Prometheus ConfigYProm
}

const (
	defaultPort      = "9157" // Reserved in the prometheus wiki for MQ
	defaultNamespace = "ibmmq"
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

	flag.StringVar(&config.httpListenPort, "ibmmq.httpListenPort", defaultPort, "HTTP Listener")
	flag.StringVar(&config.httpMetricPath, "ibmmq.httpMetricPath", "/metrics", "Path to exporter metrics")

	flag.StringVar(&config.namespace, "namespace", defaultNamespace, "Namespace for metrics")

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
				config.httpListenPort = cfy.Prometheus.Port
				config.httpMetricPath = cfy.Prometheus.MetricsPath
				config.namespace = cfy.Prometheus.Namespace
			}
		}
	}

	if err == nil {
		cf.InitLog(config.cf)
	}

	if err == nil {
		err = cf.VerifyConfig(&config.cf)
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
