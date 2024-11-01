# Changelog
Newest updates are at the top of this file.

### Nov 01 2024 (no new version)
* Move all sample container builds to use UBI base images
  * And ensure appropriate level of Go compiler is available
  * Main "Dockerfile" has RDURL_ARG build-arg option to pull Redist client 
    from an alternative location

### Oct 24 2024 (v5.6.1)
* Update to MQ 9.4.1
* Some linter-suggested code changes
* Add PasswordFile configuration option for MQ authentication
* Add note about TZ configuration (#314)
* Add an example dashboard for the OTel exporter to show any Traces sent to Jaeger

### Jun 18 2024 (v5.6.0)
* Update to MQ 9.4.0
* Update all vendored dependencies
* Add OTLP/http option to OTel emitter
  * Remove "EXPERIMENTAL" flag
* Convert OTEL and Prometheus sample dashboards to newer Grafana panels
* Update to Go 1.21 as baseline (OTEL requirement)
* Permit connection to unnamed (default) queue manager

### Mar 21 2024 (v5.5.4.1)
* Update main Dockerfile to use different base build image
  because of glibc issues when copying into regular MQ container

### Feb 29 2024 (v5.5.4)
* Update to MQ 9.3.5
* Update all vendored dependencies
* Add metadata label configuration options
* Update to Go 1.20 as baseline
* Add EXPERIMENTAL OpenTelemetry emitter

### Nov 08 2023 (v5.5.2)
* Pick up latest mq-golang fixes

### Oct 19 2023 (v5.5.1)
* Update to MQ 9.3.4
* Fix Prometheus label cardinality when reporting disconnect (#245)
* Add hints about monitoring "large" queue managers in TUNING.md
* Make sure missing YAML configuration attributes have reasonable defaults
* Update all vendored dependencies

### Jun 20 2023 (v5.5.0)
* Update to MQ 9.3.3
* Update Dockerfile to support platforms without Redist client (#209)
* Update all vendored dependencies
* Add a waitInterval config option (#227)

### Feb 17 2023 (v5.4.0)
* Update to MQ 9.3.2
* Add hostname tag for qmgrs running 9.3.2
* Add NativeHA metrics to all collectors

### Jan 10 2023 (v5.3.3)
* Copy in metrics.txt summary of available metrics from mq-golang
* JSON exporter - consistent objecttype for qmgr, merge published and
  polled metrics (#188)
* JSON exporter - allow configuration (`recordmax`) for number of objects
  printed in a single JSON record (also #188)
* AWS exporter -correct interval processing (#192)
* Simplified build/run in a single Dockerfile (#194)

### Oct 18 2022 (v5.3.2)
* Update to MQ 9.3.1
* Update to use v5.3.2 of mq-golang repository
* Update all vendored dependencies
* Add sample script showing TLS configuration in container
* Update Prometheus channel status sample dashboard to show channel instance counts
* Add AMQP channel status reporting to all collectors
* Add single CLUSTER tag to queues (ibm-messaging/mq-golang#191)

### Jul 07 2022 (v5.3.1)
* Update to use v5.3.1 of mq-golang repository

### Jun 23 2022 (v5.3.0)
* Update to MQ 9.3.0
* Update to use v5.3.0 of mq-golang repository
* Update other vendored dependencies
* Add filters.hideSvrConnJobname config option (#114)
* Move objects.showInactiveChannels to new filters section (generate error to indicate move)
- Move objects.queueSubscriptionSelection to new filters section
* Enable use of durable subscriptions to reduce impact on MAXHANDS configuration
  * Add script to explicitly remove durable subs

### Mar 10 2022 (no new version)
* Add CP4I helm charts to deploy MQ metric samples application in OCP environment.

### Mar 02 2022 (no new version)
* Add dspmqrtj application and demo

### Feb 28 2022 (no new version)
* Update to require Go 1.17 because of influxdb prereq changes

### Feb 25 2022 (v5.2.5)
* Update to MQ 9.2.5
* Update to use v5.2.5 of mq-golang repository
* Update vendored dependencies
* Permit use of local queues as well as model queues for metric replies (#100)

### Jan 16 2022 (no new version)
* Update vendored dependencies

### Nov 19 2021 (v5.2.4)
* Update to MQ 9.2.4
* Update to use v5.2.4 of mq-golang repository
* Reset vendored mq-golang/mqmetric from previous release as fixes now in base

### Sep 17 2021 (v5.2.3)
* Defaults for boolean config options not handled well by Go parser
* Add script to build images using buildah + Red Hat UBI base
* Temporarily override vendored mq-golang/mqmetric code to deliver fixes
  * Deal with buffer expansion when there are lots of queues to query AND remote system is different CCSID (#75)
  * Ensure labels - in particular object DESCR values - are valid UTF8
  * This code is newer than the upstream version which will inherit the fixes later

### Aug 03 2021 (v5.2.2)
* Update to use v5.2.2 of the mq-golang repository
* Update to use MQ 9.2.3
* Allow prometheus collector to continue running even when qmgr down (#67)
  * Report qmgr_status metric in such a situation
  * Updated "QMgr Status" sample dashboard to report both that metric and the new cluster status metric
* Update influxdb collector to use V2 APIs (#76)
  * Note changes to configuration attributes - in particular support for an APIToken
  * New sample dashboard using some very basic Flux queries

### Apr 15 2021
* Don't check for QMGR name unless config parse was a success

### Apr 8 2021
* Update Dockerfile.run to move default configuration params to environment variables

### Apr 5 2021
* Update Dockerfile.run to prevent permissions errors when running in OpenShift restricted SCC

### Mar 26 2021
* Update to use v5.2.0 of the mq-golang repository
* Update to use MQ 9.2.2
* Start to use enhanced mqmetric API that supports multiple connections (though still only having a single connection in the collectors for now)
* Permit https connection from the Prometheus engine
* Redesign collectors to permit configuration via environment variables

### Dec 04 2020
* Update to use v5.1.3 of the mq-golang repository
* Update to use MQ 9.2.1
* Update YAML processing to better handle missing values as defaults (#50)
* Add 'host' to Prometheus listener config (#51)
* URLs pointing at MQ KnowledgeCenter updated

### Sep 10 2020
* Update to use v5.1.2 of the mq-golang repository

### Aug 10 2020
* Update to use v5.1.1 of the mq-golang repository
* Moved build shell commands to `scripts` subdirectory and added README there
* Add a Windows batch file as example of building locally

### Jul 23 2020
* Update to use v5.1.0 of the mq-golang repository
* Update to use MQ 9.2.0
* Added pseudo-metric qmgr_exporter_publications for how many consumed on each scrape
* Added showInactiveChannels option (#38)
* Added explicit client configuration options (#40)

### Jun 01 2020 (v5.0.0)
* Exporters can have configuration provided in YAML file (`-f <file>`) instead of command line options
* Use modules (go.mod) to define prereqs (ibm-messaging/mq-golang#138)
* Update to use v5.0.0 of the mq-golang repository as the new module format
* Update to use more recent versions of other dependencies
* Exporters now have some specific exit codes (in particular 30 for trying to connect to a standby qmgr) (#35)
  * Default exit code for error situation is 1
* New option `-ibmmq.QueueSubscriptionFilter` to restrict subscriptions made for each queue
  * Set it to `PUT,GET` for most useful metrics (#34)
  * Default is to collect everything including OPENCLOSE and INQSET resources
* Turn off echo when asking for passwords
* All exporters now have comparable function on which objects to monitor
  * Some metric names on some exporters will have changed though not on Prometheus

### Apr 02 2020
* Update to use v4.1.4 of the mq-golang repository

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
