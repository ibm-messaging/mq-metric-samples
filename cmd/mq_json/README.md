# MQ Exporter for JSON-based monitoring

This directory contains the code for a monitoring solution
that prints queue manager data in JSON format.
It also contains configuration files to run the monitor program

The monitor collects metrics published by an MQ V9 queue manager
or the MQ appliance. The monitor program prints
these metrics to stdout.

You can see data such as disk or CPU usage, queue depths, and MQI call
counts. You can also see basic channel status information.

## Building
* This github repository contains the monitoring program and (in the
vendor tree) the ibmmq package that links to the core MQ application interface.
It also contains the mqmetric package used as a common component for
supporting alternative database collection protocols. You can use `dep ensure` to
confirm that the latest supporting packages are installed.

* You need to have the MQ client libraries installed first.
* Set up an environment for compiling Go programs
```
  export GOPATH=~/go (or wherever you want to put it)
  export GOROOT=/usr/lib/golang  (or wherever you have installed it)
  mkdir -p $GOPATH/src
  cd $GOPATH/src
```
* Clone this GitHub repository for the monitoring programs into your GOPATH. The repository
contains the prereq packages at a suitable version in the `vendor` tree
```
  git clone https://github.com/ibm-message/mq-metric-samples ibm-messaging/mq-metric-samples
```
* From the root of your GOPATH you can then compile the code
```
  cd $GOPATH
  export CGO_LDFLAGS_ALLOW='-Wl,-rpath.*'
  go build -o bin/mq_json ibm-messaging/mq-metric-samples/cmd/mq_json/*.go
```

## Configuring MQ
It is convenient to run the monitor program as a queue manager service.
This directory contains an MQSC script to define the service. In fact, the
service definition points at a simple script which sets up any
necessary environment and builds the command line parameters for the
real monitor program. As the last line of the script is "exec", the
process id of the script is inherited by the monitor program, and the
queue manager can then check on the status, and can drive a suitable
`STOP SERVICE` operation during queue manager shutdown.

Edit the MQSC script to point at appropriate directories
where the program exists, and where you want to put stdout/stderr.
Ensure that the ID running the queue manager has permission to access
the programs and output files.

Since the output from the monitor is always sent to stdout, you will
probably want to modify the script to pipe the output to a processing
program that works with JSON data, or to a program that automatically
creates and manages multiple log files.

The monitor always collects all of the available queue manager-wide metrics.
It can also be configured to collect statistics for specific sets of queues.
The sets of queues can be given either directly on the command line with the
`-ibmmq.monitoredQueues` flag, or put into a separate file which is also
named on the command line, with the `ibmmq.monitoredQueuesFile` flag. An
example is included in the startup shell script.

At each collection interval, a JSON object is printed, consisting of
a timestamp followed by an array of "points" which contain the
metric and the resource it refers to. Each point also contains tags
that further describe, and uniquely identify, the resource. All metrics for
a particular resource are printed in the same point.

For example,
```
{
   "collectionTime" : {
      "timeStamp" : "2016-11-07-T15:00:55Z"
      "epoch" : 1478527255
   },
   "points": [
   {
     "tags": {
       "qmgr": "QM1"
     },
     "metrics": {
       "userCpuTimePercentage": 5.39,
       "concurrentConnectionsHighWaterMark": 27,
       ...
     }
   },
   {
     "tags": {
       "qmgr": "QM1",
       "queue": "APP.QUEUE.1"
     },
     "metrics": {
       "destructiveMqgetNonPersistentMessage": 20,
       ...
     }
   },
   {
     "tags": {
       "channel": "TO.QM2",
       "connname": "127.0.0.1(1415)",
       "jobname": "00000F2D00000001",
       "qmgr": "QM1",
       "rqmname": "QM2",
       "type": "SENDER"
     },
     "metrics": {
       "messages": 15,
       ...
     }
   },
   ...
   ]
}

```

The JSON structure has been substantially changed in this version of
the monitor program, to give it a more deterministic layout.

As an example of processing the data, this uses the jq program to filter and show
just the channel name and the number of messages across it at each interval. It
ignores all the other metrics produced.
Simply pipe the stdout from the monitor program into this jq program.

```
jq -c '.collectionTime.timeStamp as $t | .points[] |
     select(.metrics.messages!=null) |
     {"time":$t, "chl":.tags.channel, "msg":.metrics.messages}'
```

## Metrics
Once the monitor program has been started, you will see metrics being available.
More information on the metrics collected through the publish/subscribe
interface can be found in the [MQ KnowledgeCenter]
(https://www.ibm.com/support/knowledgecenter/SSFKSJ_9.0.0/com.ibm.mq.mon.doc/mo00013_.htm)
with further description in [an MQDev blog entry]
(https://www.ibm.com/developerworks/community/blogs/messaging/entry/Statistics_published_to_the_system_topic_in_MQ_v9?lang=en)

The metrics printed are named after the
descriptions that you can see when running the amqsrua sample program, but with some
minor modifications to match a more appropriate style.

## Channel Status
The monitor program can now process channel status.

The channels to be monitored are set on the command line, similarly to
the queue patterns, with `-ibmmq.monitoredChannels` or `-ibmmq.monitoredChannelFile`.
Unlike the queue monitoring, wildcards are handled automatically by the channel
status API. So you do not need to restart this monitor in order to pick up newly-defined
channels that match an existing pattern.

Another command line parameter is `pollInterval`. This determines how frequently the
channel status is collected. You may want to have it collected at a different rate to
the queue data, as it may be more expensive to extract the channel status. The default
pollInterval is 0, which means that the channel status is collected every time the
the queue and queue manager statistics publication collection.
Setting it to `1m` means that a minimum
time of one minute will elapse between asking for channel status even if the queue statistics
are gathered more frequently.
