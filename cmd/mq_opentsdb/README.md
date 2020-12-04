# MQ Exporter for OpenTSDB monitoring

This directory contains the code for a monitoring solution
that exports queue manager data to an OpenTSDB data collection
system. It also contains configuration files to run the monitor program

The monitor collects metrics published by an MQ V9 queue manager
or the MQ appliance. The monitor program pushes
those metrics into the database, over an HTTP connection, where
they can then be queried directly or used by other packages
such as Grafana.

You can see data such as disk or CPU usage, queue depths, and MQI call
counts.

An example Grafana dashboard is included, to show how queries might
be constructed. The data shown is the same as in the corresponding
Prometheus and InfluxDB-based dashboards, also in this repository.
To use the dashboard,
create a data source in Grafana called "MQ OpenTSDB" that points at your
database server, and then import the JSON file.

## Configuring MQ
It is convenient to run the monitor program as a queue manager service.
This directory contains an MQSC script to define the service. In fact, the
service definition points at a simple script which sets up any
necessary environment and builds the command line parameters for the
real monitor program. As the last line of the script is "exec", the
process id of the script is inherited by the monitor program, and the
queue manager can then check on the status, and can drive a suitable
`STOP SERVICE` operation during queue manager shutdown.

Edit the MQSC script and the shell script to point at appropriate directories
where the program exists, and where you want to put stdout/stderr.
Ensure that the ID running the queue manager has permission to access
the programs and output files.

There are a number of required parameters to configure the service, including
the queue manager name, how to reach a database, and the frequency of reading
the queue manager publications. Look at the mq_opentsdb.sh script or config.go
to see how to provide these parameters.

The queue manager will usually generate its publications every 10 seconds. That is also
the default interval being used in the monitor program to read those publications.

## Configuring OpenTSDB
No special configuration is required for the database.

## Metrics
Once the monitor program has been started,
you will see metrics being available. Multiple series of metrics are
created, one for each type of object (queue, channel, topic etc) that is being
monitored.

The example Grafana dashboard shows how queries can be constructed to extract data
about specific object or the queue manager.

More information on the metrics collected through the publish/subscribe
interface can be found in the [MQ KnowledgeCenter](https://www.ibm.com/support/knowledgecenter/SSFKSJ_latest/com.ibm.mq.mon.doc/mo00013_.htm)
with further description in [an MQDev blog entry](https://www.ibm.com/developerworks/community/blogs/messaging/entry/Statistics_published_to_the_system_topic_in_MQ_v9?lang=en)

The metrics stored in the database are named after the
descriptions that you can see when running the amqsrua sample program, but with some
minor modifications to match a more useful style.
