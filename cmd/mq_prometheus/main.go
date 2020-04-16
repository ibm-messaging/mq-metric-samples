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
	"net/http"
	"os"

	"github.com/ibm-messaging/mq-golang/ibmmq"
	"github.com/ibm-messaging/mq-golang/mqmetric"
	cf "github.com/ibm-messaging/mq-metric-samples/pkg/config"
	"github.com/prometheus/client_golang/prometheus"
	log "github.com/sirupsen/logrus"
)

var BuildStamp string
var GitCommit string
var BuildPlatform string

func main() {
	var err error

	cf.PrintInfo("IBM MQ metrics exporter for Prometheus monitoring", BuildStamp, GitCommit, BuildPlatform)

	err = initConfig()

	// Print this because it's easy to get the escapes wrong from a shell script with wildcarded topics ('#')
	log.Debugf("Monitored topics are '%s'", config.cf.MonitoredTopics)

	if config.cf.QMgrName == "" {
		log.Errorln("Must provide a queue manager name to connect to.")
		os.Exit(1)
	}

	if err == nil {
		// Connect and open standard queues
		err = mqmetric.InitConnection(config.cf.QMgrName, config.cf.ReplyQ, &config.cf.CC)
		if err == nil {
			log.Infoln("Connected to queue manager ", config.cf.QMgrName)
			defer mqmetric.EndConnection()
		}
	}

	// What metrics can the queue manager provide? Find out, and
	// subscribe.
	if err == nil {
		// Do we need to expand wildcarded queue names
		// or use the wildcard as-is in the subscriptions
		wildcardResource := true
		if config.cf.MetaPrefix != "" {
			wildcardResource = false
		}

		mqmetric.SetLocale(config.locale)
		err = mqmetric.DiscoverAndSubscribe(config.cf.MonitoredQueues, wildcardResource, config.cf.MonitoredSTATQ, config.cf.MetaPrefix)
		mqmetric.RediscoverAttributes(ibmmq.MQOT_CHANNEL, config.cf.MonitoredChannels)
	}

	if err == nil {
		var compCode int32
		compCode, err = mqmetric.VerifyConfig()
		// We could choose to fail after a warning, but instead will continue for now
		if compCode == ibmmq.MQCC_WARNING {
			log.Println(err)
			err = nil
		}
	}

	// Once everything has been discovered, and the subscriptions
	// created, allocate the Prometheus gauges for each resource
	if err == nil {
		allocateAllGauges()
	}

	// Go into main loop for handling requests from Prometheus
	if err == nil {
		exporter := newExporter()
		prometheus.MustRegister(exporter)

		http.Handle(config.httpMetricPath, prometheus.Handler())
		http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
			w.Write(landingPage())
		})

		log.Infoln("Listening on port", config.httpListenPort)
		log.Fatal(http.ListenAndServe(":"+config.httpListenPort, nil))

	}

	if err != nil {
		log.Fatal(err)
	}

	os.Exit(0)
}

/*
landingPage gives a very basic response if someone just connects to our port.
The only link on it jumps to the list of available metrics.
*/
func landingPage() []byte {
	return []byte(
		`<html>
<head><title>IBM MQ metrics exporter for Prometheus</title></head>
<body>
<h1>IBM MQ metrics exporter for Prometheus</h1>
<p><a href='` + config.httpMetricPath + `'>Metrics</a></p>
</body>
</html>
`)
}
