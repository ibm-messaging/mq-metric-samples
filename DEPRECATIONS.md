# Deprecrations
No immediate plans for anything specific.

However it is likely that at some indeterminate future time, some of the lesser-used collectors
will be removed: aws, influx, opentsdb being most likely.

I also expect to change the Prometheus collector's default value of the `overrideCType` attribute, to
more accurately reflect the different between Counters and Gauges. That may mean reworking some of your
dashboards.
