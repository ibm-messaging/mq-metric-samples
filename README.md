# mq-metric-samples

This repository contains a collection of IBM® MQ monitoring agents that utilize
the [IBM® MQ golang metric packages](https://github.com/ibm-messaging/mq-golang)
to provide programs that can be used with existing monitoring technologies
such as Prometheus, AWS CloudWatch, etc. Statistics and status information
can be collected from queue managers and made available in databases to enable
dashboard and historic reporting.

## Health Warning

This package is provided as-is with no guarantees of support or updates.
There are also no guarantees of compatibility with any future versions of the package;
interfaces and functions are subject to change based on any feedback.

These programs use a specific version of the `mqmetric` and `ibmmq` golang packages.
Those packages are in the [mq-golang repository](https://github.com/ibm-messaging/mq-golang)
and are also included in the `vendor` tree of this repository. They are referenced in the `go.mod`
file if you wish to reload all of the dependencies by running `go mod vendor`.

## Getting started

### Requirements

You will require the following programs:

* Go compiler. This should be at least version 13.

To build the programs on Linux and MacOS, you may set an environment variable to permit some compile/link flags.
This is due to security controls in the compiler.

```
export CGO_LDFLAGS_ALLOW="-Wl,-rpath.*"
```

### Building a component

* You need to have the MQ client libraries installed first.
* Set up an environment for compiling Go programs
```
  export GOPATH=~/go (or wherever you want to put it)
  export GOROOT=/usr/lib/golang  (or wherever you have installed it)
  mkdir -p $GOPATH/src
  cd $GOPATH/src
```

* Change directory to your go path. (`cd $GOPATH`). Although GOPATH is no longer needed in the module world,
it's still a convenient variable when cloning and building the programs.
* Use git to get a copy of this repository into a new directory in the workspace:

```
git clone https://github.com/ibm-messaging/mq-metric-samples.git src/github.com/ibm-messaging/mq-metric-samples
```

* Navigate to the mq-metric-samples root directory (`$GOPATH/src/github.com/ibm-messaging/mq-metric-samples`)
* All the prereq packages are already available in the vendor directory, but you can run `go mod vendor` to reload them

* From this root directory of the repository you can then compile the code. For example,

```
  cd $GOPATH/src/github.com/ibm-messaging/mq-metric-samples
  export CGO_LDFLAGS_ALLOW='-Wl,-rpath.*'
  go build -o $GOPATH/bin/mq_prometheus ./cmd/mq_prometheus/*.go
```

At this point, you should have a compiled copy of the code in `$GOPATH/bin`. Each
monitor agent directory also has sample scripts, configuration files etc to help
with getting the agent running in your specific environment.

## Using a Docker container to build the programs
You can use the `buildMonitors.sh` script in the `scripts` subdirectory to build a Docker container that
in turn will build all the binary programs and copy them to a local directory. That script also
sets some extra version-related flags that will be shown when the program starts. The container will
automatically download and install the MQ client runtime files needed for compilation.

## Building on Windows
There is a `buildMonitors.bat` file that may help with building on Windows. It assumes you have
the [tdm-gcc-64](https://jmeubank.github.io/tdm-gcc/download/) 64-bit compiler suite installed. It
builds all the collectors and corresponding YAML configuration files into %GOPATH%/bin

## Monitor configuration
The monitors always collect all of the available queue manager-wide metrics.
They can also be configured to collect statistics for specific sets of queues where
metrics are published by the queue manager. Object status queries can be used to
extract more information about other objects such as channels and subscriptions.

The exporters can have their configuration given either on the command line
via flags, or in a YAML file described below.

### Wildcard Patterns for Queues
The sets of queues to be monitored can be given either directly on the command line with the
`-ibmmq.monitoredQueues` flag, put into a separate file which is also
named on the command line, with the `-ibmmq.monitoredQueuesFile` flag, or in the YAML configuration.

The parameter can include both positive and negative
wildcards. For example `ibmmq.monitoredQueues=A*,!AB*"` will collect data on
queues beginning with "AC" or "AD" but not "AB". The full rules for expansion can
be seen near the bottom of the _discover.go_ module in the _mqmetric_ package.

The queue patterns are expanded at startup of the program and at regular
intervals thereafter. So newly-defined queues will eventually be monitored if
they match the pattern. The rediscovery interval is 1h by default, but can be
modified by the `rediscoverInterval` parameter.

### Channel Status
The monitor programs can process channel status, reporting that back into the
database.

The channels to be monitored are set on the command line, similarly to
the queue patterns, with `-ibmmq.monitoredChannels` or `-ibmmq.monitoredChannelFile`.
Unlike the queue monitoring, wildcards are handled automatically by the channel
status API. So you do not need to restart this monitor in order to pick up newly-defined
channels that match an existing pattern. Only positive wildcards are allowed here; you cannot
explicitly exclude channels.

Another parameter is `pollInterval`. This determines how frequently the
channel status is collected. You may want to have it collected at a different rate to
the queue data, as it may be more expensive to extract the channel status. The default
pollInterval is 0, which means that the channel status is collected every time the exporter
processes the queue and queue manager resource publications. Setting it to `1m` means that a minimum
time of one minute will elapse between asking for channel status even if the queue statistics
are gathered more frequently.

A short-lived channel that connects and then disconnects in between collection intervals
will leave no trace in the status or metrics.

#### Channel Metrics
A few of the responses from the DISPLAY CHSTATUS command have been selected
as metrics. The key values returned include the status and number of messages processed.

The message count for SVRCONN channels is the number of MQI calls made by the client program.

There are actually two versions of the channel status returned. The `channel_status` metric
has the value corresponding to one of the MQCHS_* values. There are about 15 of these possible
values. There is also a `channel_status_squash` metric which returns one of only three values,
compressing the full set into a simpler value that is easier to put colours against in Grafana.
From this squashed set, you can readily see if  a channel is stopped, running, or somewhere in between.

#### Channel Instances and Labels
Channel metrics are given labels to assist in distinguishing them. These can be displayed in
Grafana or used as part of the filtering. When there is more than one instance of an active
channel, the combination of channel name, connection name and job name will be unique (though
see the z/OS section below for caveats on that platform).

The channel type (SENDER, SVRCONN etc) and the name of the remote queue manager
are also given as labels on the metric.

#### Channel Dashboard Panels
An example Grafana dashboard shows how these labels and metrics can be combined to show
some channel status from Prometheus. The Channel Status table panel demonstrates a couple of features. It
uses the labels to select unique instances of channels. It also uses a simple number-to-text
map to show the channel status as a word (and colour the cell) instead of a raw number.

The metrics for the table are selected and have '0' added to them. This may be a workround of
a Grafana bug, or it may really be how Grafana is designed to work. But without that '+0' on
the metric line, the table was showing multiple versions of the status for each channel. This
table combines multiple metrics on the same line now.

Information about channels comes from the DISPLAY CHSTATUS CURRENT command. That
only shows channels with a known state and does not report on inactive channels.
To also see the inactive channels, then set the showInactiveChannels
configuration attribute to true.

### z/OS Support
Because the DIS QSTATUS and DIS CHSTATUS commands can be used on z/OS, the monitors can now
support showing some limited information from a z/OS queue manager. There is nothing special needed to configure it, beyond the client
connectivity that allows an application to connect to the z/OS system.

The `-ibmmq.useStatus` (command line) or `useObjectStatus` (YAML) parameter must be set to `true` to use the DIS QSTATUS command.

There is also support for using the RESET QSTATS command on z/OS. This needs to be explicitly enabled
by setting the `-ibmmq.resetQStats` (command line) or `useResetQStats` (YAML) flag to true. While this option allows tracking of the number
of messages put/got to a queue (which is otherwise unavailable from z/OS queue manager status queries), it
should not be used if there are any other active monitoring solutions that are already using that command.
Only one monitor program can reliably use RESET QSTATS on a particular queue manager, to avoid the information
being split between them.

Statistics are available for pagesets and bufferpools, similar to the DISPLAY USAGE command.

#### Channel Metrics on z/OS
On z/OS, there is no guaranteed way to distinguish between multiple instances of the
same channel name. For example, multiple users of the same SVRCONN definition. On Distributed
platforms, the JOBNAME attribute does that job; for z/OS, the channel start date/time is
used in this package as a discriminator, and used as the `jobname` label in the metrics.
That may cause the stats to be slightly wrong if two instances of the same channel
are started simultaneously from the same remote address. The sample dashboard showing z/OS
status includes counts of the unique channels seen over the monitoring period.

### Authentication
Monitors can be configured to authenticate to the queue manager,
sending a userid and password.

The userid is configured using the `-ibmmq.userid` flag. The password can
be set either by using the `-ibmmq.password` flag, or by passing it via stdin.
That allows it to be piped from an external stash file or some other
mechanism. Using the command line flags for controlling passwords is not
recommended for security-sensitive environments.

Where authentication is needed for access to the database, passwords for those can
also be passed via stdin.

## YAML configuration for all exporters
Instead of providing all of the configuration for the exporters via command-line flags, you can also
provide the configuration in a YAML file. Then only the `-f` command-line option is required for the exporter to
point at the file.

All of the exporters support
the same configuration options for how to connect to MQ and which objects are monitored. There is
then an exporter-specific section for additional configuration such as how to contact the back-end
database.
The common options are shown in a template in this directory; the exporter-specific options are in individual files in each directory. Combine the two pieces into a single file to get a complete deployable configuration.

Unlike the command line flags, lists are provided in a more natural format instead of comma-separated
values in a single string. If an option is provided on both the command line and in the file, it is the file
that takes precedence. Not all strings need to be surrounded by quote characters in the file, but some (eg "!SYSTEM*")
seem to need it. The example files have used quotes where they have been found to be necessary.

The field names are slightly different in the YAML file to try to make them a bit more consistent and
structured. The command flags are not being changed to preserve compatibility with previous versions.

User passwords can be provided in the file, but it is not recommended that you do that. Instead provide the password
either on the command line or piped via stdin to the program.

## More information
Each of the sample monitor programs has its own README file describing any particular
considerations.

## History

See [CHANGELOG](CHANGELOG.md) in this directory.

## Issues and Contributions

For feedback and issues relating specifically to this package, please use the [GitHub issue tracker](https://github.com/ibm-messaging/mq-metric-samples/issues).

Contributions to this package can be accepted under the terms of the IBM Contributor License
Agreement, found in the [CLA file](CLA.md) of this repository. When submitting a pull request, you
must include a statement stating you accept the terms in the CLA.

## Copyright

© Copyright IBM Corporation 2016, 2020
