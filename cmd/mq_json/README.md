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

As an example of processing the data, this uses the jq program to filter and show
just the channel name and the number of messages across it at each interval. It
ignores all the other metrics produced.
Simply pipe the stdout from the monitor program into this jq program.

```
jq -c '.collectionTime.timeStamp as $t | .points[] |
     select(.metrics.messages!=null) |
     {"time":$t, "chl":.tags.channel, "msg":.metrics.messages}'
```

### Collector configuration options
The collector-specific options can be seen in `config.collector.yaml` in this directory.

* The `oneline` option can be used to print a complete JSON record as a single
line instead of pretty-printed across mutiple lines. That might be helpful for some tools that
ingest JSON and prefer to have a single line inputs.

* The `recordmax` option declares how many objects are printed in a single array at each interval. If
more objects have been recorded, then additional arrays are printed. This may help if processing tools
have limits on the total record size that can be handled for a single JSON object. Setting the value to `0`
means there is no effective limit.

## Metrics
Once the monitor program has been started, you will see metrics being available.
More information on the metrics collected through the publish/subscribe
interface can be found in the [MQ KnowledgeCenter](https://www.ibm.com/support/knowledgecenter/SSFKSJ_latest/com.ibm.mq.mon.doc/mo00013_.htm)
with further description in [an MQDev blog entry](https://community.ibm.com/community/user/integration/viewdocument/statistics-published-to-the-system?CommunityKey=183ec850-4947-49c8-9a2e-8e7c7fc46c64&tab=librarydocuments)

The metrics printed are named after the
descriptions that you can see when running the amqsrua sample program, but with some
minor modifications to match a more appropriate style.
