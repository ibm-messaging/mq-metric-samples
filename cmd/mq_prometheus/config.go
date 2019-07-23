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
	"bufio"
	"flag"
	"fmt"
	cf "github.com/ibm-messaging/mq-metric-samples/pkg/config"
	"os"
)

type mqExporterConfig struct {
	cf cf.Config // Common configuration attributes for all collectors

	httpListenPort string
	httpMetricPath string
	namespace      string

	locale string
}

const (
	defaultPort      = "9157" // Reserved in the prometheus wiki for MQ
	defaultNamespace = "ibmmq"
)

var config mqExporterConfig

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

	// The locale ought to be discoverable from the environment, but making it an explicit config
	// parameter for now to aid testing, to override, and to ensure it's given in the MQ-known format
	// such as "Fr_FR"
	flag.StringVar(&config.locale, "locale", "", "Locale for translated metric descriptions")

	flag.Parse()

	if len(flag.Args()) > 0 {
		err = fmt.Errorf("Extra command line parameters given")
		flag.PrintDefaults()
	}

	if err == nil {
		err = cf.VerifyConfig(&config.cf)
	}

	if err == nil {
		if config.cf.CC.UserId != "" && config.cf.CC.Password == "" {
			// TODO: If stdin is a tty, then disable echo. Done differently on Windows and Unix
			scanner := bufio.NewScanner(os.Stdin)
			fmt.Printf("Enter password: \n")
			scanner.Scan()
			config.cf.CC.Password = scanner.Text()
		}
	}

	if err == nil && config.cf.CC.UseResetQStats {
		fmt.Println("Warning: Data from 'RESET QSTATS' has been requested.")
		fmt.Println("Ensure no other monitoring applications are also using that command.\n")
	}

	if err == nil {
		cf.InitLog(config.cf)
	}

	return err
}
