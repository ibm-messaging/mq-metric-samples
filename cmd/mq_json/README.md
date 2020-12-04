# MQ Exporter for JSON-based monitoring

This directory contains the code for a monitoring solution
that prints queue manager data in JSON format.
It also contains configuration files to run the monitor program

The monitor collects metrics published by an MQ queue manager. The monitor program prints
these metrics to stdout.

You can see data such as disk or CPU usage, queue depths, and MQI call
counts. You can also see channel status information along with other
object status reports.

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
interface can be found in the [MQ KnowledgeCenter](https://www.ibm.com/support/knowledgecenter/SSFKSJ_latest/com.ibm.mq.mon.doc/mo00013_.htm)
with further description in [an MQDev blog entry](https://www.ibm.com/developerworks/community/blogs/messaging/entry/Statistics_published_to_the_system_topic_in_MQ_v9?lang=en)

The metrics printed are named after the
descriptions that you can see when running the amqsrua sample program, but with some
minor modifications to match a more appropriate style.
