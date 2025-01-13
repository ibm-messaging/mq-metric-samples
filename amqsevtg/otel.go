/*
 * This is an example of a Go program to format IBM MQ Event messages.
 * It is essentially a rewrite of the product's sample program amqsevta.c
 *
 * Functions in this file handle exporting records to an OTel logging collector. We make
 * use of the Go-provided "slog" package as the logging API and the "otelslog" package
 * which hooks into that, providing a bridge to OTel.
 */

/*
Copyright (c) IBM Corporation 2025

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the license.

	Contributors:
	  Mark Taylor - Initial Contribution
*/
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"os/signal"

	"strings"
	"time"

	mq "github.com/ibm-messaging/mq-golang/v5/ibmmq"

	resource "go.opentelemetry.io/otel/sdk/resource"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"

	otelslog "go.opentelemetry.io/contrib/bridges/otelslog"
	logGrpc "go.opentelemetry.io/otel/exporters/otlp/otlplog/otlploggrpc"
	logHttp "go.opentelemetry.io/otel/exporters/otlp/otlplog/otlploghttp"
	logStdout "go.opentelemetry.io/otel/exporters/stdout/stdoutlog"
	logsdk "go.opentelemetry.io/otel/sdk/log"
)

var (
	ctx = context.Background()

	logProvider *logsdk.LoggerProvider
	logger      *slog.Logger
)

const (
	programName = "amqsevtg"

	// Define how the batchProcessor will behave
	exportMaxBatchSize  = 100
	exportTimeoutString = "2s"
)

func initOTel() error {

	res, err := resource.Merge(resource.Default(),
		resource.NewWithAttributes(
			semconv.SchemaURL,
			semconv.ServiceName(programName), // Might prefer to have the qmgr name in here?
			semconv.ServiceVersion(mq.MQIGolangVersion()),
		))
	if err != nil {
		return err
	}

	logProvider, err = newLoggerProvider(ctx, res)
	if err != nil {
		return err
	}

	_, stop := signal.NotifyContext(ctx, os.Interrupt)
	defer stop()

	return nil
}

// Called at the end of the program to flush any remaining records
func shutdownOTel() {
	fmt.Fprintf(statusStream, "Shutting down OTel logging\n")
	logProvider.ForceFlush(ctx)
	logProvider.Shutdown(ctx)
}

// Create an exporter for some known exporter types and link it to the logger
//   - Stdout
//     endpoint is "stdout"
//   - OTLP/GRPC is an alternative when you define an endpoint
//     endpoint is a simple hostname:port string
//   - OTEL/HTTP can be used if you set the endpoint to start with the protocol
//     endpoint is http[s]://hostname:port
//
// Additional config options for TLS (certs etc) can come via OTEL-defined env vars
func newLoggerProvider(ctx context.Context, res *resource.Resource) (*logsdk.LoggerProvider, error) {

	var err error
	var exporter logsdk.Exporter

	endpoint := cf.OTelEndpoint

	if endpoint == "stdout" {
		fmt.Fprintln(statusStream, "Creating STDOUT exporter")
		exporter, err = logStdout.New()
		if err != nil {
			return nil, err
		}
	} else if strings.HasPrefix(endpoint, "http://") || strings.HasPrefix(endpoint, "https://") {
		opts := []logHttp.Option{
			logHttp.WithEndpointURL(endpoint),
		}

		fmt.Fprintf(statusStream, "Creating OTLP/http exporter for %s\n", endpoint)
		if cf.OTelInsecure {
			fmt.Fprintf(statusStream, "Enabling insecure mode for HTTP\n")
			opts = append(opts, logHttp.WithInsecure())
		}

		exporter, err = logHttp.New(ctx, opts...)
		if err != nil {
			return nil, err
		}
	} else {
		opts := []logGrpc.Option{
			logGrpc.WithEndpoint(endpoint),
		}

		if cf.OTelInsecure {
			fmt.Fprintf(statusStream, "Enabling insecure mode for GRPC\n")
			opts = append(opts, logGrpc.WithInsecure())
		}

		fmt.Fprintf(statusStream, "Creating OTLP/gRPC exporter for %s\n", endpoint)

		exporter, err = logGrpc.New(ctx, opts...)
		if err != nil {
			return nil, err
		}
	}

	// Setup the BatchProcessor
	exportTimeout, _ := time.ParseDuration(exportTimeoutString)
	bp := logsdk.NewBatchProcessor(exporter,
		logsdk.WithExportTimeout(exportTimeout),
		logsdk.WithExportMaxBatchSize(exportMaxBatchSize))

	// Setup the LoggerProvider
	lp := logsdk.NewLoggerProvider(
		logsdk.WithResource(res),
		logsdk.WithProcessor(bp),
	)

	slog.SetLogLoggerLevel(slog.LevelInfo)

	// Create a logger, and link it to the OTel LogProvider
	logger = otelslog.NewLogger(programName,
		otelslog.WithLoggerProvider(lp),
		otelslog.WithVersion(mq.MQIGolangVersion()))

	return lp, nil

}

// Write the event message as a log record.
// We'll just take the default mechanisms provided by otelslog.
//
// One useful extension in particular would
// be a way to set the Timestamp on the log record based on the
// Event message contents. There might be a way to do that via
// attributes processors in the OTel Collector but it's getting
// beyond the scope of this sample. Just in case, we'll add the
// timestamp as an explicit attribute anyway.
func otelLog(event Event) {
	b, _ := json.Marshal(event)
	logger.LogAttrs(ctx, slog.LevelInfo, string(b),
		slog.String("eventCreation", event.EventCreation.TimeStamp),
	)
}
