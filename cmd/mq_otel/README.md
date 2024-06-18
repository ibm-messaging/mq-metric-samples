# MQ Exporter for OpenTelemetry monitoring

This README should be read in conjunction with the repository-wide
[README](https://github.com/ibm-messaging/mq-metric-samples/blob/master/README.md) that covers features common to all of
the collectors in this repository.

This directory contains an implementation of a tool to send MQ metrics to an OpenTelemetry system. The
reported metrics are the same as for the other collectors in this package.

## Collectors
OpenTelemetry supports a range of protocols and implementations, with a common format for the metrics. (We are not
concerned here with the other aspects - tracing, logging - that OpenTelemetry also deals with.) Tools can be chained
together to process metrics in some way (eg filtering, aggregation) before passing them onwards for storage and
visualisation. We do no extra processing here; the metrics emitted are the complete set of raw data.

Three collector types are enabled: `stdout` (the default), `otlp/grpc` and `otlp/http`. To use OTLP, set the `endpoint`
configuration attribute. For example,
```
otel:
  endpoint: localhost:4317
```
in the YAML file. The GRPC component has few additional configuration options enabled; other options can be 
applied via environment variables - in particular, for TLS.

To use OTLP over HTTP, then the endpoint needs to be a fuller URL. For example, `http://localhost:4318`.

The OpenTelemetry Collector is often going to be the initial destination for metrics; it will then forward them to any
of a large range of available backends.

One full path to visualisation of metrics that I have tested goes

```
  queue manager -> mq_otel reader -> OpenTelemetry Collector -> Prometheus -> Grafana
```

### Additional OTLP Configuration options 
   
There are a number of environment variables that can be used to configure the OTLP exporters, without needing explicit
configuration in this package. See *otlp/otlpmetric/otlpmetricgrpc/config.go* for more details in the OpenTelemetry
package source code. But they include:

- `OTEL_EXPORTER_OTLP_CERTIFICATE`
- `OTEL_EXPORTER_OTLP_CLIENT_CERTIFICATE`
- `OTEL_EXPORTER_OTLP_CLIENT_KEY`
- `OTEL_EXPORTER_OTLP_COMPRESSION`
- `OTEL_EXPORTER_OTLP_ENDPOINT`
- `OTEL_EXPORTER_OTLP_HEADERS`
- `OTEL_EXPORTER_OTLP_INSECURE`
- `OTEL_EXPORTER_OTLP_TIMEOUT`
- `OTEL_EXPORTER_OTLP_METRICS_CLIENT_CERTIFICATE`
- `OTEL_EXPORTER_OTLP_METRICS_CLIENT_KEY`
- `OTEL_EXPORTER_OTLP_METRICS_COMPRESSION`
- `OTEL_EXPORTER_OTLP_METRICS_ENDPOINT`
- `OTEL_EXPORTER_OTLP_METRICS_HEADERS`
- `OTEL_EXPORTER_OTLP_METRICS_INSECURE`
- `OTEL_EXPORTER_OTLP_METRICS_TIMEOUT`


For test purposes, you can get the default OpenTelemetry Collector program running in a container with
```
  v=latest
  docker pull otel/opentelemetry-collector:$v      
  docker run -it -p 127.0.0.1:4317:4317 -p 127.0.0.1:55679:55679 otel/opentelemetry-collector:$v   
```

### Exporting from the OTel Collector to Prometheus

The OTel Collector can route metrics to a variety of systems. One of the available exporters is Prometheus. In this
mode, you configure the Prometheus server to pull data from the OTel Collector. In this example, the exposed port number
is 9159.

The OTel Collector configuration file can also be set to preserve metric
names:
```
exporters:
   prometheus:
    endpoint: ':9159'
    send_timestamps: true
    resource_to_telemetry_conversion:
      enabled: false
    add_metric_suffixes: false
```

## Counters and Gauges
This tool follows the OpenTelemetry rules on the distinction between Counters and Gauges. A Counter is something that
increases monotonically. For example, the number of messages that have been put to a queue. While a Gauge is something
that can go down as well as up. For example, the number of connections to a queue manager. This has implications for
viewing metrics. See [here](https://www.innoq.com/en/blog/2019/05/prometheus-counters/) for a good introduction to
viewing Prometheus Counters.

For historic reasons, the Prometheus collector that is also in this repository reports all metrics as Gauges by default.
There is now an option to also have a mixture of Counters and Gauges but enabling that is an incompatible change. 

The difference means that any dashboards that have been built using the current Prometheus model may need to change
queries/style if they are going to deal with metrics reported through this OpenTelemetry path.

If you want to use existing dashboards with minimal changes, then you can set the `overrideCType` configuration
attribute to `true`; that causes this collector to report everything as Gauges instead of the mixture of types.

A sample Grafana dashboard in this directory shows a couple of ways of looking at counters.

## Timestamps
Note that the timestamps in the data are derived from when they were sent from this tool to the OTel Collector, rather
than when they were read from the queue manager or when the reading cycle started. So they might have small variations
between the timestamps, rather than having all metrics in a given scrape appearing with an identical value.


## Metric Naming
Metrics are initially named like `ibmmq.channel.buffers_sent`. The middle element is the object type: queue, channel,
topic etc. Attributes are attached to each instance of each metric, with values such as the object name and the queue
manager name. These are the same as the tags or labels associated with the metrics in the other collectors. Other
components in the path may choose to rename or reformat the metrics slightly, to match the expected format. There might,
for example, be a `_count` or `_total` added to the name.

## Additional configuration
Apart from any GRPC options, there might need to be further config such as naming the "service". This will depend on
feedback from any users.
