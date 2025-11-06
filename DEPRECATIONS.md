# Deprecration Notices

## 2025-11-05
* I expect that at the next full update of this repo (probably to coincide with the next MQ LTS release), 
some of the lesser-used collectors will be removed: aws, influx, opentsdb being most likely.
  * The growing use of OpenTelemetry means that format/protocol can be used in many scenarios where you might
    previously have needed a specific client.

* I also expect to change the Prometheus collector's default value of the `overrideCType` attribute at the
same time, to more accurately reflect the different between Counters and Gauges. That may mean reworking
some of your dashboards.
