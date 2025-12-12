# mq-metric-samples

This repository contains a collection of IBM MQ monitoring agents that utilize the
[IBM MQ Go metric packages](https://github.com/ibm-messaging/mq-golang) to provide programs that can be used with
existing monitoring technologies such as Prometheus, AWS CloudWatch, etc. It can also send data to an OpenTelemetry
environment. Statistics and status information can be collected from queue managers and made available in databases to
enable dashboard and historic reporting.

## The dspmqrtj program
The repository also includes a program which traces the route a message can take through the MQ network. It is similar
to the `dspmqrte` program that is part of the MQ product, but writes the output in JSON format. See the `dspmqrtj`
subdirectory for more information.

## Health Warning

This package is provided as-is with no guarantees of support or updates. There are also no guarantees of compatibility
with any future versions of the package; interfaces and functions are subject to change based on any feedback. You
cannot use IBM formal support channels (Cases/PMRs) for assistance with material in this repository.

This does not affect the status of the underlying MQ C client libraries which have their own support conditions.

These programs use a specific version of the `mqmetric` and `ibmmq` golang packages. Those packages are in the
[mq-golang repository](https://github.com/ibm-messaging/mq-golang) and are also included in the `vendor` tree of this
repository. They are referenced in the `go.mod` file if you wish to reload all of the dependencies by running
`go mod vendor`.

See the [DEPRECATIONS](DEPRECATIONS.md) file for any planned changes to the metric collectors.

## Getting started

### Requirements

You will require the following programs:

* Go compiler - version 1.21 is the minimum defined here
* C compiler

### MQ Client SDK

The MQ Client SDK for C programs is required in order to compile and run Go programs. You may have this from an MQ
Client installation image (eg rpm, deb formats for Linux, msi for Windows).

For Linux x64 and Windows systems, you may also choose to use the MQ Redistributable Client package which is a simple
zip/tar file that does not need any privileges to install:

* Download
  [IBM MQ redistributable client](https://public.dhe.ibm.com/ibmdl/export/pub/software/websphere/messaging/mqdev/redist)
* Unpack archive to fixed directory. E.g. `c:\IBM-MQC-Redist-Win64` or `/opt/mqm`.

See the README file in the mq-golang repository for more information about any environment variables that may be
required to point at non-default directories for the MQ C SDK.

### Building a component on your system directly

* You need to have the MQ client libraries installed first.
* Create a directory where you want to work with the programs.
* Change to that directory.
* Use git to get a copy of this repository into a new directory in the workspace. For example:

```
git clone https://github.com/ibm-messaging/mq-metric-samples.git src/github.com/ibm-messaging/mq-metric-samples
```

* Navigate to the mq-metric-samples root directory (`./src/github.com/ibm-messaging/mq-metric-samples`)
* All the prereq packages are already available in the vendor directory, but you can run `go mod vendor` to reload them

* From this root directory of the repository you can then compile the code. For example,

```
  cd ./src/github.com/ibm-messaging/mq-metric-samples
  export CGO_LDFLAGS_ALLOW='-Wl,-rpath.*'
  mkdir -p /tmp/go/bin
  go build -mod=vendor -o /tmp/go/bin/mq_prometheus ./cmd/mq_prometheus/*.go
```

At this point, you should have a compiled copy of the code in `/tmp/go/bin`. Each monitor agent directory also has
sample scripts, configuration files etc to help with getting the agent running in your specific environment.
The `-mod=vendor` option is important so that the build process does not need to download additional files from external

repositories.

## Using containers to build the programs
The `Dockerfile` in the root directory gives a simple way to both build and run a collector program through containers.
You still need to provide the configuration file at runtime, perhaps as a mounted volume. For example:

```
  docker build -t mqprom:1.0 .
  docker run   -p 9157:9157 -v <directory>/mq_prometheus.yaml:/opt/config/mq_prometheus.yaml mqprom:1.0
```

### Platform support
This Dockerfile should work for a variety of platforms. For those with a Redistributable client, it uses `curl` to
automatically download and unpack the required MQ files. For other platforms, it assumes that you have an `MQINST`
subdirectory under this root, and then copied the `.rpm` or `deb` files (or the `.tar.gz` file for Linux/arm64 systems)
from your real MQ installation tree into it. The base OS image in the Dockerfile is rpm-based, but you might want to
change it to your preferred base container image.

### Additional container scripts

As a more flexible example, you can use the `buildMonitors.sh` script in the `scripts` subdirectory to build a Docker
container that in turn will build all the binary programs and copy them to a local directory. That script also sets some
extra version-related flags that will be shown when the program starts. The container will automatically download and
install the MQ client runtime files needed for compilation. This might be a preferred approach when you want to run a
collector program alongside a queue manager (perhaps as an MQ SERVICE) and you need to copy the binaries to the target
system.

## Building to run on Windows
There is a `buildMonitors.bat` file to help with building on Windows. It assumes
* You have the [msys2](https://www.msys2.org/) 64-bit GCC compiler suite installed into the `C:\msys64` directory. The
  specific compiler version from this package can be installed with `pacman -S mingw-w64-ucrt-x86_64-gcc`. That should
  end up with the `gcc` command being available.
* You have set the GOPATH environment variable to the root of your source trees. For example, `C:\Gowork`.

The script builds all the collectors and corresponding YAML configuration files into %GOPATH%/bin. An alternative
compiler could be [tdm-gcc-64](https://jmeubank.github.io/tdm-gcc/download/)

## Queue manager configuration
When metrics are being collected from the publish/subscribe interface (all platforms except z/OS), there are some
considerations:
* MAXHANDS on queue manager: The default configuration of these collectors uses non-durable subscriptions to get
  information about queue metrics. Each subscription uses an object handle. If many queues are being monitored the
  default MAXHANDS may need to be increased. A warning is printed if the monitor thinks this attribute appears too low.
  See below for an alternative option.
* MAXDEPTH on model queues: The model queue used as the basis for publication and reply queues in the monitor must have
  a MAXDEPTH suitable for the expected amount of data. For published metrics, this is estimated based on holding one
  minute's amount of publications; the number of monitored channels is also used as an estimate, although that does not
  need to be time-based as the data is requested directly by the monitor.
* USEDLQ on the admin topic: The USEDLQ attribute on the topic object associated with the metrics publications (usually
  `SYSTEM.ADMIN.TOPIC`) determines what happens if the subscriber's queue is full. You might prefer to set this to NO to
  avoid filling the system DLQ if the collection program does not read the publications frequently enough.

### Local and client connections
Connections to the queue manager can be made with either local or client bindings. Running the collector "alongside" the
queue manager is usually preferred, with the collector configured to run as a service. Sample scripts in this repository
show how to define an appropriate MQ SERVICE. Client connections can be made by specifying the channel and connName
information in the basic configuration; in this mode, only plaintext communication is available (similar to the MQSERVER
environment variable). For secure communication using TLS, then you must provide connection information via a CCDT. Use
the `ccdtUrl` configuration option or environment variables to point at a CCDT that can be in either binary or JSON
format. The `runMonitorTLS.sh` script gives a simple example of setting up a container to use TLS.

### Using durable subscriptions
An alternative collection mechanism uses durable subscriptions for the queue metric data. This may avoid needing to
increase the MAXHANDS attribute on a queue manager. (Queue manager-level metrics are still collected using non-durable
subscriptions.)

To set it up, you must provide suitable configuration options. In the YAML configuration, these are the attributes
(command line or environment variable equivalents also exist):
- `replyQueue` must refer to a local queue (not a model queue)
- `replyQueue2` must also be set, referring to a different local queue
- `durableSubPrefix` is a string that is unique across any collectors that might be connected to this queue manager

If you use durable subscriptions, then the named reply queues may continue to receive publications even when the
collector is not running, so that may induce queue-full reports in the error log or events. The subscriptions can be
manually removed using the `DELETE SUB()` MQSC command for all subscriptions where the subscription ids begin with the
`durableSubPrefix` value. The `scripts/cleanDur.sh` program can be used for this deletion. You should also clean the
subscriptions when the configuration of which data to collect has changed, particularly the `queueSubscriptionSelector`
option. Any publications generated while the the collector is not running are, in any case, discarded on startup of
the collector.

## Monitor configuration
The monitors always collect all of the available queue manager-wide metrics. They can also be configured to collect
statistics for specific sets of queues where metrics are published by the queue manager. Object status queries can be
used to extract more information about other objects such as channels and subscriptions.

The exporters can have their configuration given on the command line via flags, via environment variables, or in a YAML
file described below.

### Wildcard Patterns for Queues
The sets of queues to be monitored can be given either directly on the command line with the `-ibmmq.monitoredQueues`
flag, put into a separate file which is also named on the command line, with the `-ibmmq.monitoredQueuesFile` flag, or
in the equivalent YAML configuration.

The parameter can include both positive and negative wildcards. For example `ibmmq.monitoredQueues=A*,!AB*"` will
collect data on queues beginning with "AC" or "AD" but not "AB". The full rules for expansion can be seen near the
bottom of the _discover.go_ module in the _mqmetric_ package.

The queue patterns are expanded at startup of the program and at regular intervals thereafter. So newly-defined queues
will eventually be monitored if they match the pattern. The rediscovery interval is 1h by default, but can be modified
by the `rediscoverInterval` parameter.

### Queue Manager Statistics Events
An experimental alternative to the published resource statistics for queue and many queue manager metrics is to use the
Statistics Events messages. The collection of metrics from these events has been enabled for the OpenTelemetry, JSON and
Prometheus collectors only.

These events are available on the Distributed platforms by setting the queue manager `STATMQI` attribute to `ON` and
also enabling the `STATQ` attribute for the queue manager and/or individual queues. You should also set the `STATINT`
value to something appropriate for your collection interval. The default value of 1800 (30 minutes) is probably much too
large. The `STATCHL` option is ignored; any Event messages reporting on channel activity are ignored as the channel
status responses already give similar metrics.

Set the `global.useStatistics` value to `true` in the collector's configuration to use this option. The default value is
`false`. You can also use the `connection.StatisticsQueue` option to name the queue holding these events if it is not
the standard `SYSTEM.ADMIN.STATISTICS.QUEUE`. You should still have `global.usePublications` set to `true` if you want
to collect other metrics - for example, the queue manager logger details, or NativeHA metrics that are only available
through the publication route. So there will still be some subscriptions, but to a much smaller number of topics.

When you use these events, the `monitoredQueues` value is overridden and automatically set to `*`. All queues that have
the `STATQ` enabled, either explicitly or through inheritance from the queue manager setting, are reported on. We
continue to use some published metrics, primarily at the queue manager level, but there are no longer queue-specific
subscriptions. Instead the values are taken from the Event messages.

The metrics created through this mechanism may have different names than similar metrics that are returned from the
publications. The Events may not report on every queue at every interval; it may only be queues that have had some
activity during the interval. Regardless of the metric names, the actual set of metrics is not identical across the two
approaches - generally, the published metrics can go into more detail on queue activity than the corresponding STATQ
events. However some metrics can be found only in one of the sets, and some only in the other.

If you look at the PCF definition of these event messages, you will see that many of them return arrays of metrics. For
example there are separate values returned in the same element for the persistent and non-persistent MQPUT message
counts. The collection process here simplifies the aggregation of these elements by creating and reporting an extra
value of the total across the array. This should simplify dashboards so you do not need to query for multiple metrics,
only to add them up yourself. But the individual numbers are still available too.

Look at the metrics.txt file to see all of the metrics available through this mechanism.

If you are using the Prometheus collector, the `overrideCType` value is automatically set to `true`, which gives the
correct distinction between Counters and Gauges. See the Prometheus directory's README for more information on that
attribute.

The seemingly-similar Accounting Events are **not** handled in these collectors.

### Channel Status
The monitor programs can process channel status, reporting that back into the database.

The channels to be monitored are set on the command line, similarly to the queue patterns, with
`-ibmmq.monitoredChannels` or `-ibmmq.monitoredChannelFile`. Unlike the queue monitoring, wildcards are handled
automatically by the channel status API. So you do not need to restart this monitor in order to pick up newly-defined
channels that match an existing pattern. Only positive wildcards are allowed here; you cannot explicitly exclude
channels.

Another parameter is `pollInterval`. This determines how frequently the channel status is collected. You may want to
have it collected at a different rate to the queue data, as it may be more expensive to extract the channel status. The
default pollInterval is 0, which means that the channel status is collected every time the exporter processes the queue
and queue manager resource publications. Setting it to `1m` means that a minimum time of one minute will elapse between
asking for channel status even if the queue statistics are gathered more frequently.

A short-lived channel that connects and then disconnects in between collection intervals will leave no trace in the
status or metrics.

#### Channel Metrics
Some the responses from the DISPLAY CHSTATUS command have been selected as metrics. The key values returned include the
status and number of messages processed.

The message count for SVRCONN channels is the number of MQI calls made by the client program.

There are actually two versions of the channel status returned. The `channel_status` metric has the value corresponding
to one of the MQCHS_* values. There are about 15 of these possible values. There is also a `channel_status_squash`
metric which returns one of only three values, compressing the full set into a simpler value that is easier to put
colours against in Grafana. From this squashed set, you can readily see if a channel is stopped, running, or somewhere
in between.

#### Channel Instances and Labels
Channel metrics are given labels to assist in distinguishing them. These can be displayed in Grafana or used as part of
the filtering. When there is more than one instance of an active channel, the combination of channel name, connection
name and job name will be unique (though see the z/OS section below for caveats on that platform).

The channel type (SENDER, SVRCONN etc) and the name of the remote queue manager are also given as labels on the metric.

#### Channel Dashboard Panels
An example Grafana dashboard shows how these labels and metrics can be combined to show some channel status from
Prometheus. The Channel Status table panel demonstrates a couple of features. It uses the labels to select unique
instances of channels. It also uses a simple number-to-text map to show the channel status as a word (and colour the
cell) instead of a raw number.

The metrics for the table are selected and have '0' added to them. This may be a workround of a Grafana bug, or it may
really be how Grafana is designed to work. But without that '+0' on the metric line, the table was showing multiple
versions of the status for each channel. This table combines multiple metrics on the same line now.

Information about channels comes from the DISPLAY CHSTATUS CURRENT command. That only shows channels with a known state
and does not report on inactive channels. To also see the inactive channels, then set the showInactiveChannels
configuration attribute to true.

### NativeHA
When NativeHA is used, the queue manager publishes some metrics on its status. These are automatically collected
whenever available, and can be seen in the metric lists. The metrics are given a prefix or series of "nha". For example,
`ibmmq_nha_synchronous_log_sent_bytes` is one metric shown in Prometheus. The NativeHA "instance" or "group"
name - given to the replicas - is added as the `nha` tag to the metrics. Each NativeHA metric
is associated with either an instance or group, never both.

Note: The `nha` tag was previously called `nhainstance` but
that was confusing when the cross-region replication feature was introduced. Existing dashboards might need updating
to show the newer tag.

One type of metric related to NativeHA show a Log Sequence Number (LSN). In the queue manager, these will be shown
in a style such as `GRPLSN(<0:0:326:39203>)`. Grafana does not have an inbuilt formatter to convert the integer into
this style; you can display the raw value, but it's difficult to see the meaning. Other tools might permit additional processing.

Depending on configuration, the collector may be able to automatically reconnect to the new instance after a failover.
If that is not possible, you will need to have a process to restart the collector once the new replica has taken over.

### z/OS Support
Because the DIS QSTATUS and DIS CHSTATUS commands can be used on z/OS, the monitors support showing some information
from a z/OS queue manager. There is nothing special needed to configure it, beyond the client connectivity that allows
an application to connect to the z/OS system.

The `-ibmmq.useStatus` (command line) or `useObjectStatus` (YAML) parameter must be set to `true` to use the DIS QSTATUS
command.

There is also support for using the RESET QSTATS command on z/OS. This needs to be explicitly enabled by setting the
`-ibmmq.resetQStats` (command line) or `useResetQStats` (YAML) flag to true. While this option allows tracking of the
number of messages put/got to a queue (which is otherwise unavailable from z/OS queue manager status queries), it should
not be used if there are any other active monitoring solutions that are already using that command.

Only one monitor program can reliably use RESET QSTATS on a particular queue manager, to avoid the information being
split between them.

Statistics are available for pagesets and bufferpools, similar to the DISPLAY USAGE command.

#### Channel Metrics on z/OS
On z/OS, there is no guaranteed way to distinguish between multiple instances of the same channel name. For example,
multiple users of the same SVRCONN definition. On Distributed platforms, the JOBNAME attribute does that job; for z/OS,
the channel start date/time is used in this package as a discriminator, and used as the `jobname` label in the metrics.
That may cause the stats to be slightly wrong if two instances of the same channel are started simultaneously from the
same remote address. The sample dashboard showing z/OS status includes counts of the unique channels seen over the
monitoring period.

### Authentication
Monitors can be configured to authenticate to the queue manager, sending a userid and password.

The userid is configured using the `-ibmmq.userid` flag. The password can be set either by using the `-ibmmq.password`
flag, or by passing it via stdin. That allows it to be piped from an external stash file or some other mechanism. Using
the command line flags for controlling passwords is not recommended for security-sensitive environments.

You can also use the `-ibmmq.passwordFile` where the file contains a plaintext password. This might be appropriate for
container environments where secrets are mounted into the running container.

Where authentication is needed for access to a database, passwords for those can also be passed via stdin.

### Timezones
For some configurations, especially when running the exporters as client-connected rather than directly alongside
the queue manager, you may see warnings about timezone offsets. Perhaps the exporter is in a different timezone
than the queue manager. One particular warning would be: "Status reports appear to be from the future".

The `tzOffset` configuration attribute may help to deal with this - it indicates the difference between the queue manager
timezone and the exporter's.

When running components in a container, you may also wish to adjust the container's timezone for consistency. This may
need you to set the `TZ` environment variable for the container. You may also need to set timezone data files for the
container as many standard container base images do not do that. See
[here](https://access.redhat.com/solutions/2567961) for some ways to do that. The `Workaround 3` has been
[reported](https://github.com/ibm-messaging/mq-metric-samples/issues/314) as being effective.


## YAML configuration for all exporters
Instead of providing all of the configuration for the exporters via command-line flags, you can also provide the
configuration in a YAML file. Then only the `-f` command-line option is required for the exporter to point at the file.

All of the exporters support the same configuration options for how to connect to MQ and which objects are monitored.
There is then an exporter-specific section for additional configuration such as how to contact the back-end database.
The common options are shown in a template in this directory; the exporter-specific options are in individual files in
each directory. Combine the two pieces into a single file to get a complete deployable configuration.

Unlike the command line flags, lists are provided in a more natural format instead of comma-separated values in a single
string. If an option is provided on both the command line and in the file, it is the file that takes precedence. Not all
strings need to be surrounded by quote characters in the file, but some (eg "!SYSTEM*") seem to need it. The example
files have used quotes where they have been found to be necessary.

The field names are slightly different in the YAML file to try to make them a bit more consistent and structured. The
command flags are not being changed to preserve compatibility with previous versions.

User passwords can be provided in the file, but it is not recommended that you do that. Instead provide the password
either on the command line or piped via stdin to the program.

## Environment variable configuration for all exporters
As a further alternative for configuration, parameters can be set by environment variables. This may be more convenient
when running collectors in a container as the variables may be easier to modify for each container than setting up
different YAML files. The names of the variables follow the YAML naming pattern with an IBMMQ prefix, underscore
separators, and in uppercase.

For example, the queue manager name can be set with `IBMMQ_CONNECTION_QUEUEMANAGER`. You can use the "-h" parameter to
the collector to see the complete set of options.

### Precedence of configuration options
The command line flags are highest precedence. Environment variables override settings in the YAML file, And the YAML
overrides the hardcoded default values.


## Metadata tags
For all the collectors, you can configure additional metadata that is replicated into the tags or labels produced on
each metric. These might indicate, for example, whether a queue manager is DEV or PROD level. These are defined using
the `-ibmmq.metadataTags` and `-ibmmq.metadataValues` command line flags (comma-separated), corresponding
`IBMMQ_CONNECTION`-level environment variables, or as arrays within the YAML configuration file.

## More information
Each of the sample monitor programs has its own README file describing any particular
considerations. The metrics.txt file in this directory has a summary of the available
metrics for each object type.

## History

See [CHANGELOG](CHANGELOG.md) in this directory.

## Issues and Contributions

For feedback and issues relating specifically to this package, please use the [GitHub issue tracker](https://github.com/ibm-messaging/mq-metric-samples/issues).

Contributions to this package can be accepted under the terms of the Developer's Certificate
of Origin, found in the [DCO file](DCO1.1.txt) of this repository. When
submitting a pull request, you must include a statement stating you accept the terms
in the DCO.

## Copyright

© Copyright IBM Corporation 2016, 2024
