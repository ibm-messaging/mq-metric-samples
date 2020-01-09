# Changelog
Newest updates are at the top of this file.

### Jan 09 2020 
* Update to use v4.1.3 of the mq-golang repository
* mqmetric - Add DESCR attribute from queues and channels to permit labelling in metrics (ibm-messaging/mq-metric-samples#16)

### Dec 06 2019
* Update to use v4.1.2 of the mq-golang repository
* Collect some channel configuration attributes
  * Supported in the Prometheus monitor

### Jul 11 2019
* Update to use v4.0.8 of the mq-golang repository
* Enable use of USAGE queue attribute
  * Supported in the Prometheus and JSON monitors
* Don't give a visible error from setmqenv if queue manager remote

### Jun 24 2019
* Update to use v4.0.7 of the mq-golang repository
* Enable time-based re-expansion of queue wildcards while monitors are running
  * `-rediscoverInterval=30m` Default is `1h`. Can be set to 0 to disable.
  * Supported in the Prometheus and JSON monitors
* Show z/OS pageset/bufferpool data
  * Supported in the Prometheus and JSON monitors

### Jun 06 2019
* Update to use v4.0.6 of the mq-golang repository
* Permit limited monitoring of pre-V9 Distributed platforms
* Add `queue_attribute_max_depth` to permit %full calculation

## April 23 2019
* Update to use v4.0.5 of the mq-golang repository
* Add ability to set timezone offset between collector and qmgr

## April 08 2019
* Add subscription status
* Add topic and queue manager status support from latest mqmetric library
* Add Grafana/Prometheus dashboards showing the newer metrics
* Update InfluxDB collector to similar level as Prometheus/JSON

## March 2019
* Update to use v4.0.1 of the mq-golang repository
* Update READMEs for all the monitor agent programs
* Add a Dockerfile and scripts to build the agent binaries in a container

## November 2018
* Added "platform" as a tag/label in Prometheus and JSON exporters

## November 2018
* Updated the JSON monitor sample to support channel status reporting
* Updated Prometheus and JSON monitors to deal with DISPLAY QSTATUS commands
* Updated to permit access to z/OS status data.

## October 2018
* Updated the Prometheus monitor program to support channel status reporting
* Updated the README for Prometheus for newer build instructions
* Updated dependencies

## July 2018
* Updated repo to use 2.0.0 version of mq-golang repo

## July 2018
* Added templates for PR and Issues

## May 2018
* Initial commit
* Added Golang dependency management using [dep](https://golang.github.io/dep/)
