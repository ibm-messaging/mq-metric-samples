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
	"context"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"time"

	"github.com/ibm-messaging/mq-golang/v5/ibmmq"
	"github.com/ibm-messaging/mq-golang/v5/mqmetric"
	cf "github.com/ibm-messaging/mq-metric-samples/v5/pkg/config"

	otel "go.opentelemetry.io/otel"

	exportGrpc "go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetricgrpc"
	exportStdout "go.opentelemetry.io/otel/exporters/stdout/stdoutmetric"

	metricotel "go.opentelemetry.io/otel/metric"
	metricsdk "go.opentelemetry.io/otel/sdk/metric"
	metricdata "go.opentelemetry.io/otel/sdk/metric/metricdata"
	"go.opentelemetry.io/otel/sdk/resource"
	semconv "go.opentelemetry.io/otel/semconv/v1.21.0"

	"github.com/go-logr/stdr"
	log "github.com/sirupsen/logrus"
)

var BuildStamp string
var GitCommit string
var BuildPlatform string
var discoverConfig mqmetric.DiscoverConfig

var (
	ctx  context.Context
	stop context.CancelFunc

	totalErrorCount = 0

	res = resource.NewWithAttributes(
		semconv.SchemaURL,
		semconv.ServiceName("ibmmq"),
		semconv.ServiceVersion("0.0.1" /*cf.MqGolangVersion()*/), // A dummy version for now to indicate experimental
	)

	meterProvider *metricsdk.MeterProvider
	exporter      metricsdk.Exporter
	meter         metricotel.Meter

	metricReader metricsdk.Reader
)

func main() {
	var err error
	var d time.Duration

	cf.PrintInfo("IBM MQ metrics exporter for OpenTelemetry monitoring", BuildStamp, GitCommit, BuildPlatform)

	err = initConfig()

	if err == nil && config.cf.QMgrName == "" {
		log.Errorln("Must provide a queue manager name to connect to.")
		os.Exit(72)
	}

	// OTel libraries use the stdr logger. Let's try to make it match the loglevel
	// of this MQ package.
	verbosity := 0
	if config.ci.LogLevel != "" {
		logrusLevel, _ := log.ParseLevel(config.ci.LogLevel)
		switch logrusLevel {
		case log.ErrorLevel:
			verbosity = 0
		case log.WarnLevel:
			verbosity = 1
		case log.InfoLevel:
			verbosity = 4
		case log.DebugLevel:
			verbosity = 8
		case log.TraceLevel:
			verbosity = 8
		}
	}
	stdr.SetVerbosity(verbosity)

	if err == nil {
		d, err = time.ParseDuration(config.ci.Interval)
		if err != nil || d.Seconds() <= 1 {
			log.Errorln("Invalid or too short value for interval parameter: ", err)
			os.Exit(1)
		}

		// Connect and open standard queues
		err = mqmetric.InitConnection(config.cf.QMgrName, config.cf.ReplyQ, config.cf.ReplyQ2, &config.cf.CC)
	}
	if err == nil {
		log.Infoln("Connected to queue manager ", config.cf.QMgrName)
	} else {
		if mqe, ok := err.(mqmetric.MQMetricError); ok {
			mqrc := mqe.MQReturn.MQRC
			mqcc := mqe.MQReturn.MQCC
			if mqrc == ibmmq.MQRC_STANDBY_Q_MGR {
				log.Errorln(err)
				os.Exit(30) // This is the same as the strmqm return code for "active instance running elsewhere"
			} else if mqcc == ibmmq.MQCC_WARNING {
				log.Infoln("Connected to queue manager ", config.cf.QMgrName)
				// Report the error but allow it to continue
				log.Errorln(err)
				err = nil
			}
		}
	}

	if err == nil {
		defer mqmetric.EndConnection()
	}

	// What metrics can the queue manager provide? Find out, and
	// subscribe.

	if err == nil {
		discoverConfig.MonitoredQueues.ObjectNames = config.cf.MonitoredQueues
		discoverConfig.MonitoredQueues.UseWildcard = true
		discoverConfig.MonitoredQueues.SubscriptionSelector = strings.ToUpper(config.cf.QueueSubscriptionSelector)
		discoverConfig.MetaPrefix = config.cf.MetaPrefix
		err = mqmetric.DiscoverAndSubscribe(discoverConfig)
		mqmetric.RediscoverAttributes(ibmmq.MQOT_CHANNEL, config.cf.MonitoredChannels)
		mqmetric.RediscoverAttributes(mqmetric.OT_CHANNEL_AMQP, config.cf.MonitoredAMQPChannels)

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

	if err == nil {
		mqmetric.ChannelInitAttributes()
		mqmetric.QueueInitAttributes()
		mqmetric.TopicInitAttributes()
		mqmetric.SubInitAttributes()
		mqmetric.QueueManagerInitAttributes()
		mqmetric.UsageInitAttributes()
		mqmetric.ClusterInitAttributes()
		mqmetric.ChannelAMQPInitAttributes()
	}

	// Set up access to the OpenTelemetry components
	if err == nil {
		exporter, err = newExporter()
		if err != nil {
			log.Fatal(err)
		}

		// Some MQ metrics come out as truly cumulative (eg channel message count). But we convert all counter metrics to deltas.
		deltaTemporalitySelector := func(metricsdk.InstrumentKind) metricdata.Temporality { return metricdata.DeltaTemporality }

		metricReader = metricsdk.NewManualReader(metricsdk.WithTemporalitySelector(deltaTemporalitySelector))
		meterProvider = metricsdk.NewMeterProvider(metricsdk.WithResource(res), metricsdk.WithReader(metricReader))
		defer func() {
			err := meterProvider.Shutdown(context.Background())
			if err != nil {
				log.Fatal(err)
			}
		}()

		deepDebug("MetricReader: %+v", metricReader)
		deepDebug("Exporter is %+v", exporter)
		deepDebug("MeterProvider: %+v", meterProvider)

		otel.SetMeterProvider(meterProvider)
		meter = meterProvider.Meter("ibmmq")

		ctx, stop = signal.NotifyContext(context.TODO(), os.Interrupt)
		defer stop()
	}

	if err == nil {

		// For debug purposes, we may want to end the collector after a short time
		loopCount := 0
		maxLoops := 0
		ml := os.Getenv("OTEL_MAXLOOPS")
		if ml != "" {
			maxLoops, _ = strconv.Atoi(ml)
		}

		for {
			rm := metricdata.ResourceMetrics{}
			// deepDebug("Initial rm: %+v", rm)

			err = GetMetrics(ctx, meter)
			if err == nil {
				// This will drive the callbacks for gauges that are stashed during the GetMetrics call
				err = metricReader.Collect(ctx, &rm)
				if err == nil {
					// We should now have everything available in a structure. So push it to the collector
					// deepDebug("About to write rm: %+v", rm)
					err = exporter.Export(ctx, &rm)
				}
			}

			if err != nil {
				log.Errorf("Collection error: %v", err)
				totalErrorCount++
			} else {
				totalErrorCount = 0
			}

			if totalErrorCount > config.ci.MaxErrors {
				log.Fatal("Too many errors communicating with server")
			}
			log.Debugf("Error counts: global %d", totalErrorCount)

			loopCount++
			if maxLoops > 0 && loopCount > maxLoops {
				log.Fatalf("Exiting after %d proper collection loop(s)", maxLoops)
			}

			time.Sleep(d)
		}

	}

	if err != nil {
		log.Fatal(err)
	}

	os.Exit(0)
}

func newExporter() (metricsdk.Exporter, error) {
	// Returns an exporter for a couple of known exporter types.
	// - Stdout is the default (endpoint is not defined)
	// - OTLP/GRPC is the alternative when you define an endpoint
	// Additional config options will need to be provided for the GRPC protocol, but
	// this is sufficient to demonstrate something. Though they can also come via OTEL-defined env vars
	if config.ci.Endpoint == "" {
		return exportStdout.New()
	} else {
		opts := []exportGrpc.Option{
			exportGrpc.WithEndpoint(config.ci.Endpoint),
		}

		insecure := cf.AsBool(config.ci.Insecure, false)
		if insecure {
			log.Debugf("Enabling insecure mode for GRPC")
			opts = append(opts, exportGrpc.WithInsecure())
		}
		return exportGrpc.New(ctx, opts...)
	}
}

// The spew package gives a deeper inspection of object, that
// was helpful in initial debug. Leaving it here for now, but
// commented out.
func deepDebug(s string, obj interface{}) {
	log.Debugf(s, obj)
	/*
		if log.GetLevel() >= log.DebugLevel {
			spew.Fdump(os.Stderr, obj)
		}
	*/
}
