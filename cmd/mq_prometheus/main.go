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
	"context"
	"crypto/tls"
	"net/http"
	"os"

	"strings"
	"sync"
	"time"

	"github.com/ibm-messaging/mq-golang/v5/ibmmq"
	"github.com/ibm-messaging/mq-golang/v5/mqmetric"
	cf "github.com/ibm-messaging/mq-metric-samples/v5/pkg/config"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	log "github.com/sirupsen/logrus"
)

var (
	BuildStamp     string
	GitCommit      string
	BuildPlatform  string
	discoverConfig mqmetric.DiscoverConfig
	usingTLS       = false
	server         *http.Server
	startChannel   = make(chan bool)
	collector      prometheus.Collector
	mutex          sync.RWMutex
	retryCount     = 0 // Might use this with a maxRetry to force a quit out of collector
)

func main() {
	var err error
	cf.PrintInfo("IBM MQ metrics exporter for Prometheus monitoring", BuildStamp, GitCommit, BuildPlatform)

	err = initConfig()

	if err == nil && config.cf.QMgrName == "" {
		log.Errorln("Must provide a queue manager name to connect to.")
		os.Exit(72) // Same as strmqm "queue manager name error"
	}

	if err != nil {
		log.Error(err)
	} else {
		setConnectedOnce(false)
		setConnectedQMgr(false)
		setCollectorEnd(false)
		setFirstCollection(false)
		setCollectorSilent(false)

		// Start the webserver in a separate thread
		go startServer()

		// This is the main loop that tries to keep the collector connected to a queue manager
		// even after a failure.
		for !isCollectorEnd() {
			log.Debugf("In main loop: qMgrConnected=%v", isConnectedQMgr())
			err = nil // Start clean on each loop

			// The callback will set this flag to false if there's an error while
			// processing the messages.
			if !isConnectedQMgr() {
				mutex.Lock()
				if err == nil {
					mqmetric.EndConnection()
					// Connect and open standard queues. If we're going to manage reconnection from
					// this collector, then turn off the MQ client automatic option
					if config.keepRunning {
						config.cf.CC.SingleConnect = true
					} else {
						config.cf.CC.SingleConnect = false
					}
					err = mqmetric.InitConnection(config.cf.QMgrName, config.cf.ReplyQ, config.cf.ReplyQ2, &config.cf.CC)
					if err == nil {
						log.Infoln("Connected to queue manager " + config.cf.QMgrName)
					} else {
						if mqe, ok := err.(mqmetric.MQMetricError); ok {
							mqrc := mqe.MQReturn.MQRC
							mqcc := mqe.MQReturn.MQCC
							if mqrc == ibmmq.MQRC_STANDBY_Q_MGR {
								log.Errorln(err)
								os.Exit(30) // This is the same as the strmqm return code for "active instance running elsewhere"
							} else if mqcc == ibmmq.MQCC_WARNING {
								log.Infoln("Connected to queue manager " + config.cf.QMgrName)
								log.Errorln(err)
								err = nil
							}
						}
					}
				}

				if err == nil {
					retryCount = 0
					defer mqmetric.EndConnection()
				}

				// What metrics can the queue manager provide? Find out, and subscribe.
				if err == nil {
					// Do we need to expand wildcarded queue names
					// or use the wildcard as-is in the subscriptions
					wildcardResource := true
					if config.cf.MetaPrefix != "" {
						wildcardResource = false
					}

					mqmetric.SetLocale(config.cf.Locale)
					discoverConfig.MonitoredQueues.ObjectNames = config.cf.MonitoredQueues
					discoverConfig.MonitoredQueues.SubscriptionSelector = strings.ToUpper(config.cf.QueueSubscriptionSelector)
					discoverConfig.MonitoredQueues.UseWildcard = wildcardResource
					discoverConfig.MetaPrefix = config.cf.MetaPrefix
					err = mqmetric.DiscoverAndSubscribe(discoverConfig)
					mqmetric.RediscoverAttributes(ibmmq.MQOT_CHANNEL, config.cf.MonitoredChannels)
					log.Debugf("Returned from RediscoverAttributes with error %v", err)
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
				// created, allocate the Prometheus gauges for each resource. If this is
				// a reconnect, then we clean up and create a new collector with new gauges
				if err == nil {
					allocateAllGauges()

					if collector != nil {
						setCollectorSilent(true)
						prometheus.Unregister(collector)
						setCollectorSilent(false)
					}

					collector = newExporter()
					setFirstCollection(true)
					prometheus.MustRegister(collector)
					setConnectedQMgr(true)

					if !isConnectedOnce() {
						startChannel <- true
						setConnectedOnce(true)
					}
				} else {
					if !isConnectedOnce() || !config.keepRunning {
						// If we've never successfully connected, then exit instead
						// of retrying as it probably means a config error
						log.Errorf("Connection to %s has failed. %v", config.cf.QMgrName, err)
						setCollectorEnd(true)
					} else {
						log.Debug("Sleeping a bit after a failure")
						retryCount++
						time.Sleep(config.reconnectIntervalDuration)
					}
				}
				mutex.Unlock()
			} else {
				log.Debug("Sleeping a bit while connected")
				time.Sleep(config.reconnectIntervalDuration)
			}
		}
	}
	log.Info("Done.")

	if err != nil {
		os.Exit(10)
	} else {
		os.Exit(0)
	}
}

func startServer() {
	var err error
	// This function starts a new thread to handle the web server that will then run
	// permanently and drive the exporter callback that processes the metric messages

	// Need to wait until signalled by the main thread that it's setup the gauges
	log.Debug("HTTP server - waiting until MQ connection ready")
	<-startChannel

	http.Handle(config.httpMetricPath, promhttp.Handler())
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Write(landingPage())
	})

	address := config.httpListenHost + ":" + config.httpListenPort
	if config.httpsKeyFile == "" && config.httpsCertFile == "" {
		usingTLS = false
	} else {
		usingTLS = true
	}

	if usingTLS {
		// TLS has been enabled for the collector (which is acting as a TLS Server)
		// So we setup the TLS configuration from the keystores and let Prometheus
		// contact us over the https protocol.
		cert, err := tls.LoadX509KeyPair(config.httpsCertFile, config.httpsKeyFile)
		if err == nil {
			server = &http.Server{Addr: address,
				Handler: nil,
				// More fields could be added here for further control of the connection
				TLSConfig: &tls.Config{
					Certificates: []tls.Certificate{cert},
					MinVersion:   tls.VersionTLS12,
				},
			}
		} else {
			log.Fatal(err)
		}
	} else {
		server = &http.Server{Addr: address,
			Handler: nil,
		}
	}

	// And now we can start the protocol-appropriate server
	if usingTLS {
		log.Infoln("Listening on https address", address)
		err = server.ListenAndServeTLS("", "")
		if err != nil && err != http.ErrServerClosed {
			log.Fatalf("Metrics Error: Failed to handle metrics request: %v", err)
			stopServer()
		}
	} else {
		log.Infoln("Listening on http address", address)
		err = server.ListenAndServe()
		log.Fatalf("Metrics Error: Failed to handle metrics request: %v", err)
		stopServer()
	}
}

// Shutdown HTTP server
func stopServer() {
	timeout, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	err := server.Shutdown(timeout)
	if err != nil {
		log.Errorf("Failed to shutdown metrics server: %v", err)
	}
	setCollectorEnd(true)
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
