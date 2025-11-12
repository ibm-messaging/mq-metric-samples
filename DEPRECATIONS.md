# Deprecration Notices

This file lists features that are being considered for removal or for a breaking configuration change in the
`mq-metric-samples` repository.

## Nov 2025

Note that these deprecated features will prompt a major version change in this repository. That will mean it no
longer has the same version number as the underlying `mq-golang` packages.

The update of this repo where these take effect will probably coincide with the next MQ LTS release.

* Some of the lesser-used collectors will be removed: aws, influx, opentsdb being most likely.
  * The growing use of OpenTelemetry means that format/protocol can be used in many scenarios where you might
    previously have needed a specific client.

* The Prometheus collector's default value of the `overrideCType` attribute will change to `true`, to
  more accurately reflect the difference between Counters and Gauges. That may mean reworking some of your dashboards.

* The `useObjectStatus` configuration attribute will be removed and will effectively be permanently enabled because a
  number of other capabilities essentially require it to be active. Instead, configuration of the monitored objects
  (channels, topics etc) can be independently controlled if you wish to reduce the number of `DIS xxSTATUS` commands
  issued.
