# dspmqrtj - IBM MQ routing information in JSON

This package is a way to show the path that messages can take through an MQ network.
It is very similar to the **dspmqrte** command that is part of the product, but creates its
output in JSON format to make it easier to work with in automated environments.

This is a mostly-new implementation. It was initially derived, not from the **dspmqrte** program
but from the TraceRoute plugin written in Java as part of the SupportPac MS0P extensions for the
MQ Explorer. Though as the coding progressed, it's probably now hard to see any relationship to
that original Java.

## Prerequisites
* A Go compiler
* MQ client SDK (libraries and header files)

Other dependencies are included in the `vendor` tree or can be refreshed with the `go mod vendor` command.

## How to use it

It is similar to **dspmqrte** - [here](https://www.ibm.com/docs/en/ibm-mq/latest?topic=reference-dspmqrte-display-route-information)
is that program's documentation - but without all of the options. I wanted to just get to the basics of displaying the
route. At its simplest, you can
run `dspmqrtj -m QM1 -q LOOP.ALIAS` where the designated queue happens to be the start of a chain.

### Options
Use the `-?` flag to see all the options.

The program can connect as a client to queue managers on other machines. Simple configuration (channel name, connName)
can be provided on the command line; more complex configurations such as TLS channels would usually require use of a CCDT
which can also be named in the command parameters.

There is one particular option that this program has, like its Java/Eclipse predecessor, but is not in the product **dspmqrte**: the
ability to send multiple traces as part of a single command. This can aid validation or diagnostics of MQ Cluster workload distributions.
In a default configuration, sending 10 messages to a cluster where 3 of the queue managers host instances of the same queue should result
in each queue manager getting 3 or 4 messages.

You can also trace the progress of publications by using the `-t` option and giving a topic string (not an administered topic object name).

## Building the program
The Go dependencies are all included in the `vendor` tree of the repository. All you then need is an MQ client package installed somewhere.
At its simplest, running `go build --mod=vendor -o dspmqrtj *.go` should create an executable program. See the instructions in
[the MQ Go library](https://github.com/ibm-messaging/mq-golang) for more complex configurations, especially if the MQ code is not installed in the
standard locations.

## Demonstration scripts
The `demo` directory contains scripts to demonstrate use of the package.
* The `crtClus.sh` script creates a set of 6 queue managers in a cluster with some objects
that get used by the demo. You might want to edit this script, for example to modify the
port numbers used by the channel listeners (they start from 4714)
* The `demo.sh` script runs the program in several ways
* The `dltClus.sh` script stops and deletes the queue managers.

## Program output
The whole point of this program is to create JSON-formatted output. And you can then use any JSON-aware processor on it.

The output will look something like:

```
{
  "summary": {
    "parameters": "-m QM1 -q LOOP.ALIAS",
    "success": "Message successfully reached a destination",
    "totalTimeMilliSec": 0,
    "channelCount": 2,
    "finalQueueManager": "QM1"
  },
  "activities": [
    {
      "applName": "dspmqrtj,
      "applType": "Unix",
      "description": "Display Route Application (GO/JSON Version)",
      "operations": [
        {
          "operation": "Put",
          "details": {
            "localQueueManager": "QM1",
            "objectName": "LOOP.ALIAS",
            "objectType": "queue",
            "remoteQueue": "LOOP.RETURN",
            "remoteQueueManager": "QM2",
            "resolvedQueue": "QM2"
          }
        }
      ]
    },
    {
      "applName": "runmqchl",
      "applType": "Unix",
      "description": "Sending Message Channel Agent",
      "operations": [
        {
          "operation": "Get",
          "time": "08:55:42.56",
          "epochMilliSec": 1644396942560,
          "details": {
            "queue": "QM2",
            "queueManager": "QM1",
            "resolvedQueue": "QM2"
       }

...

       {
         "operation": "Discard",
         "time": "08:55:42.56",
         "epochMilliSec": 1644396942560,
         "details": {
           "feedback": "NotDelivered",
           "queue": "LOOP.OUTPUT",
           "queueManager": "QM1"
         }
       }
     ],
     "traceRoute": {
       "discontinuityCount": 0,
       "maxActivities": 100,
       "recordedActivities": 4,
       "routeAccumulation": "None",
       "routeDelivery": "No",
       "routeDetail": "Medium",
       "routeForwarding": "All",
       "unrecordedActivites": 0
     }
   }
 ]
}
```

## The summary section
This will indicate whether or not the tracer message reached a final destination and the reported total
time for the flow. It also shows how many channels were traversed.

### Cluster distribution

One example of JSON processing is around cluster distribution testing. Assuming QM1 does not have an
instance of Q1, but other cluster members do, then extracting the *finalQueueManager* field
shows the destination. We would then hope to see something like this example, which removes unnecessary
details in a simple filter:

```
   $ dspmqrtj -m QM1 -q Q1 -msgCount=10 | jq -r '.summary.finalQueueManager' | sort | uniq -c
   3 PR1
   4 PR2
   3 PR3
```

### Errors
Errors show up in an error section of the JSON. Where appropriate, MQCC and MQRC values are included.

```
$ dspmqrtj -m QM1 --maxWait=3s -q XXX
{
  "summary": {
    "parameters": "-m QM1 --maxWait=3s -q XXX",
    "totalTimeMilliSec": 0
  },
  "error": {
    "description": "Cannot open queue XXX",
    "detailedError": "MQOPEN: MQCC = MQCC_FAILED [2] MQRC = MQRC_UNKNOWN_OBJECT_NAME [2085]",
    "mqcc": 2,
    "mqrc": 2085
  }
}
```

If the tracer message does not reach a final destination, then there might not be an error message. But
the summary section still shows that it was not completely successful:

```
summary": {
    "parameters": "-m QM1 --maxWait=3s -q R.QM2.STOPPED",
    "failure": "Message not reported at a final destination",
    "totalTimeMilliSec": 0
  },
```
Typical reasons for this include a channel being down - either on the outbound or return paths - or a queue manager being
configured to not return responses. Any responses that are included in the report should help narrow down where the problem is.

### Time stamps
Timing of operations is displayed both as strings showing the recorded time, and as an epoch value in milliseconds.

## Health Warning

This package is provided as-is with no guarantees of support or updates. You cannot use
IBM formal support channels (Cases/PMRs) for assistance with material in this repository.

## History

See [CHANGELOG](CHANGELOG.md) in this directory.

## Copyright

© Copyright IBM Corporation 2022
