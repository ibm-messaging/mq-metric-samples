# MQ Exporter for InfluxDB monitoring

This directory contains the code for a monitoring solution
that exports queue manager data to an InfluxDB data collection
system. It also contains configuration files to run the monitor program

The current version of the collector is designed using the InfluxDB V2 APIs
which are different from the V1 API and configuration parameters. 

This uses a token-based authentication method rather than userid/password. 
Influx claim that the V2 APIs can still connect to a V1 database using an apiToken of 
a format that combines the userid and password.

The monitor collects metrics published by an MQ V9 queue manager
or the MQ appliance. The monitor program pushes
those metrics into the database, over an HTTP connection, where
they can then be queried directly or used by other packages
such as Grafana.

You can see data such as disk or CPU usage, queue depths, and MQI call
counts.

Example Grafana dashboards are included, to show how queries might
be constructed. The graphs shown are similar to the corresponding
Prometheus-based dashboard, also in this repository.

To use the dashboard,
create a data source in Grafana called "MQ Influx" that points at your
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
the queue manager publications. Look at the mq_influx.sh script or the yaml
configuration file to see how to provide these parameters.

The queue manager will usually generate its publications every 10 seconds. That is also
the default interval being used in the monitor program to read those publications.

## Configuring InfluxDB
No special configuration is required for InfluxDB. You may want to create an MQ-specific
bucket, and suitable userids with API tokens to access that bucket.

## Metrics
Once the monitor program has been started,
you will see metrics being available.

The example Grafana dashboard shows how queries can be constructed to extract data
about specific objects or the queue manager.

More information on the metrics collected through the publish/subscribe
interface can be found in the [MQ KnowledgeCenter](https://www.ibm.com/support/knowledgecenter/SSFKSJ_latest/com.ibm.mq.mon.doc/mo00013_.htm)
with further description in [an MQDev blog entry](https://community.ibm.com/community/user/integration/viewdocument/statistics-published-to-the-system?CommunityKey=183ec850-4947-49c8-9a2e-8e7c7fc46c64&tab=librarydocuments)

The metrics stored in the Influx database are named after the
descriptions that you can see when running the amqsrua sample program, but with some
minor modifications to match a more useful style.
