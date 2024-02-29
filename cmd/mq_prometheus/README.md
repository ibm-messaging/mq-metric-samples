# MQ Exporter for Prometheus monitoring

This README should be read in conjunction with the repository-wide
[README](https://github.com/ibm-messaging/mq-metric-samples/blob/master/README.md) that covers features common to all of
the collectors in this repository.

This directory contains the code for a monitoring solution that exports queue manager data to a Prometheus data
collection system. It also contains configuration files to run the monitor program

The monitor collects metrics published by an MQ queue manager. Prometheus than calls the monitor
program at regular intervals to pull those metrics into its database, where they can then be queried directly or used by
other packages such as Grafana.

You can see data such as disk or CPU usage, queue depths, and MQI call counts. Channel status is also reported.

Example Grafana dashboards are included, to show how queries might be constructed. To use the dashboard, create a data
source in Grafana called "MQ Prometheus" that points at your database server, and then import the JSON file. This
dashboard was originally built using Grafana v5.3.1

## Configuring MQ
It is convenient to run the monitor program as a queue manager service whenever possible.

This directory contains an MQSC script to define the service. In fact, the service definition points at a simple script
which sets up any necessary environment and builds the command line parameters for the real monitor program. As the last
line of the script is "exec", the process id of the script is inherited by the monitor program, and the queue manager
can then check on the status, and can drive a suitable `STOP SERVICE` operation during queue manager shutdown.

Edit the MQSC script and the shell script to point at appropriate directories where the program exists, and where you
want to put stdout/stderr. Ensure that the ID running the queue manager has permission to access the programs and output
files.

If you cannot run the monitor as a service, for example when trying to monitor the MQ Appliance which does not support
service definitions, or monitoring another platform without Go support, then you can run as an MQ client connecting
remotely. Setting the `ibmmq.client` property to `true` forces client connections. Then all the usual MQ configuration
comes into play (MQSERVER environment variable, use of CCDT files etc).

The monitor listens for calls from Prometheus on a TCP port. The default port, reserved for MQ's use in the Prometheus
list, is 9157. If you want to use a different number, then use the `-ibmmq.httpListenPort` command parameter.

## Configuring Prometheus
The Prometheus server has to know how to contact the MQ monitor. The simplest way is just to add a reference to the
monitor in the server's configuration file. For example, by adding this block to /etc/prometheus/prometheus.yml.

```
  # Adding a reference to an MQ monitor. All we have to do is
  # name the host and port on which the monitor is listening.
  # Port 9157 is the reserved default port for this monitor.
  - job_name: 'ibmmq'
          scrape_interval: 15s

    static_configs:
          - targets: ['hostname.example.com:9157']
```

The server documentation has information on more complex options, including the ability to pull information on which
hosts should be monitored from a variety of discovery tools.

### Prometheus TLS
The Prometheus servers have the capability to use TLS to contact collector programs such as this. If you set the
`httpsCertFile` and `httpsKeyFile` configuration parameters, the https protocol is enabled for the collector instead of
using an unprotected http connection. This is the most basic TLS capability; additional function could be added in
future, for example to validate the Prometheus system's credentials.

Note that in TLS terms, the Prometheus engine is considered a **client** as it is the component that initiates the
http(s) connection, while this collector is a **server**.

## Metrics
Once the monitor program has been started, and Prometheus refreshed to connect to it, you will see metrics being
available in the prometheus console. All of the metrics are given the a prefix which by default is **ibmmq**. The name
can be configured with the `-namespace` option on the command line.

More information on the metrics collected through the publish/subscribe
interface can be found in the [MQ KnowledgeCenter](https://www.ibm.com/docs/en/ibm-mq/latest?topic=trace-metrics-published-system-topics)
with further description in [an MQDev blog entry](https://community.ibm.com/community/user/integration/viewdocument/statistics-published-to-the-system?CommunityKey=183ec850-4947-49c8-9a2e-8e7c7fc46c64&tab=librarydocuments)

The queue and queue manager metrics shown in the Prometheus console are named after the descriptions that you can see
when running the amqsrua sample program, but with some minor modifications to match the required style.

The metrics for other object types all begin with the type of that object.

## Unavailable queue managers
If the queue manager is not available, the collector can be configured to continually attempt to reconnect with the
`keepRunning` parameter (provided that it was available and successfully connected once). In this mode, the web server
called by Prometheus to give the metrics continues to run. It will return a single metric `qmgr_status` indicating that
the queue manager is down. This may be the preferred execution model when the collector is not running as a queue
manager service.