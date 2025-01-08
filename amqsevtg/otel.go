/*
 * This is an example of a Go program to format IBM MQ Event messages.
 * It is essentially a rewrite of the product's sample program amqsevta.c
 *
 * Functions in this file handle exporting records to an OTel logging collector
 */

/*
Copyright (c) IBM Corporation 2024

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
	"os"
	"os/signal"
	"strings"

	otelslog "go.opentelemetry.io/contrib/bridges/otelslog"
        slog "log/slog"

	"go.opentelemetry.io/otel/log/global"

	log "go.opentelemetry.io/otel/log"
	logsdk "go.opentelemetry.io/otel/sdk/log"

	resource "go.opentelemetry.io/otel/sdk/resource"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"

	logGrpc "go.opentelemetry.io/otel/exporters/otlp/otlplog/otlploggrpc"
	logHttp "go.opentelemetry.io/otel/exporters/otlp/otlplog/otlploghttp"
	logStdout "go.opentelemetry.io/otel/exporters/stdout/stdoutlog"
    
        //spew "github.com/spewerspew/spew" 
)

var (
	ctx = context.Background()

	logProvider *logsdk.LoggerProvider
	exporter    logsdk.Exporter
	logger      *slog.Logger           
)

func initOTel() error {

	res, err := resource.Merge(resource.Default(),
		resource.NewWithAttributes( 
                        semconv.SchemaURL,
                        semconv.ServiceName("amqsevtg"),
			semconv.ServiceVersion("0.1.0"),
		))
	if err != nil {
		return err
	}

	logProvider, err = newLoggerProvider(ctx, res)
	if err != nil {
		return err
	}

/*
	defer func() {
                fmt.Fprintf(statusStream, "Shutting down\n")
                logProvider.ForceFlush(ctx)
		err := logProvider.Shutdown(ctx)
		if err != nil {
			logger.Error(err.Error())
		}
	}()
*/
	fmt.Fprintf(statusStream, "Exporter      : %+v\n", exporter)
	fmt.Fprintf(statusStream, "LoggerProvider: %+v\n", logProvider)

        //spew.Fdump(statusStream,logger)
        //spew.Fdump(statusStream,exporter)

	_, stop := signal.NotifyContext(ctx, os.Interrupt)
	defer stop()

	return nil
}

func newLoggerProvider(ctx context.Context, res *resource.Resource) (*logsdk.LoggerProvider, error) {

	var err error
	endpoint := OTelEndpoint

	// Returns an exporter for a couple of known exporter types.
	// - Stdout  
	// - OTLP/GRPC is an alternative when you define an endpoint
	//   endpoint is a simple hostname:port string
	// - OTEL/HTTP can be used if you set the endpoint to start with the protocol
	//   endpoint is http[s]://hostname:port
	// Additional config options for TLS could be provided for the GRPC protocol, but
	// they can also come via OTEL-defined env vars
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
		if OTelInsecure {
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

		if OTelInsecure {
			fmt.Fprintf(statusStream, "Enabling insecure mode for GRPC\n")
			opts = append(opts, logGrpc.WithInsecure())
		}

		fmt.Fprintf(statusStream, "Creating OTLP/gRPC exporter for %s\n", endpoint)

		exporter, err = logGrpc.New(ctx, opts...)
		if err != nil {
			return nil, err
		}
	}

	fmt.Fprintf(statusStream, "Resources: %+v\n", res)
        bp := logsdk.NewBatchProcessor(exporter,logsdk.WithExportMaxBatchSize(2))
	lp := logsdk.NewLoggerProvider(
		logsdk.WithResource(res),
		logsdk.WithProcessor(bp),
	)

	global.SetLoggerProvider(lp)
        slog.SetLogLoggerLevel(slog.LevelInfo)
        logger = otelslog.NewLogger("amqsevt",otelslog.WithLoggerProvider(lp),otelslog.WithVersion("0.1.1"))

	return lp,  nil

}

func otelLog(event Event) {
	b, _ := json.Marshal(event)
//        fmt.Printf("%s\n",b)
	logger.Log(ctx,slog.LevelInfo, string(b))
	//logger.LogAttrs(ctx,slog.LevelInfo, string(b), slog.Group("EV",event))
 //       r:=convert(b)
 //       err := exporter.Export(ctx,r)
 //       fmt.Printf("Exporter error: %+v\n",err)
}

func convert(b []byte) []logsdk.Record {
  var ra []logsdk.Record
  var r logsdk.Record 
  r.SetBody(log.StringValue(string(b)))
  ra = append(ra,r)
  return ra
}

