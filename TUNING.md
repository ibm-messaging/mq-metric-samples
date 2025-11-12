# Tuning hints for monitoring "large" queue managers

If you have a large queue manager - perhaps several thousands of queues - then a lot of data could be produced for
monitoring those queues. Some default configuration options might need tuning to get acceptable performance. Reducing
the frequency of generation and/or collection may be appropriate. There are several places where tuning might be
done: in this collector, in the database configuration, and in the queue manager.

The following sections describe different pieces that you might want to look at.

The document is mostly written from the viewpoint of using Prometheus as the database. That is mainly because
Prometheus has the unique "pull" model, where the server calls the collector at configured intervals. Other databases and
collector technologies supported from this repository have a simpler way of "pushing" data to the various backends.
However much of the document is relevant regardless of where the metrics end up.

## Collector location
It is most efficient to run the collector program as a local bindings application, connecting directly to the queue
manager. That removes all the MQ Client flows that would have to be done for every message.

If you cannot avoid running as a client (for example, you are trying to monitor the MQ Appliance or z/OS), then keep the
network latency between the queue manager and collector as low as possible. For z/OS, you might consider running the
collector in a zLinux LPAR on the same machine. Or perhaps in a zCX container.

If you are running as a client, then configure it to take advantage of readahead when getting publications. This is done by
setting `DEFREADA(YES)` on the nominated ReplyQueue(s).

## Collection processing time
The collector reports on how long it takes to collect and process the data on each interval. You can see this in a debug
log. The Prometheus collector also has a `ibmmq_qmgr_exporter_collection_time` metric. Note that this time is the value
as seen by the main collection thread; the real total time as seen by Prometheus is usually longer. This is because there
is likely still work going on in the background to send metrics to the database, and for it to be successfully ingested.

The first time that the collection time exceeds the Prometheus default `scrape_timeout` value, a warning message is
emitted. This can be ignored if you are expecting a scrape to take a longer period. But it can be helpful if you didn't
know that you might need to do some tuning.

The true total time taken for a scrape can be seen in Prometheus directly. For example, you can use the administrative
interface at `http://<server>:9090/targets?search=` and find the target corresponding to your queue manager.

For other collectors, there is no specific metric. But the timestamps on each collection block allow you to deduce the
time taken as the difference between successive iterations is the collection period plus the `interval` configuration
value.

## Ensuring collection intervals have enough time to run
The Prometheus `scrape_configs` configuration attributes can be configured for all or some collectors. In particular,
you will probably want to change the `scrape_interval` and `scrape_timeout` values for the jobs associated with large
queue managers. Use the reported collection processing time as a basis from which to set these values.

For other collector models, the collector-specific `interval` attribute determines the gap between each push of the
metrics. There is no "maximum" collection time.

## Reducing metric publication interval from queue manager
By default, the queue manager publishes resource metrics every 10 seconds. This matches fairly well with the Prometheus
default scrape interval of 15s. But if you increase the scrape interval, you might also want to reduce the frequency of
publications so that fewer "merges" have to be done when processing the subscription destination queues. Setting the
following stanza in the _qm.ini_ file changes that frequency:
```
   TuningParameters:
     MonitorPublishHeartBeat = 30
```
This value is given in seconds. And the attribute is case-sensitive. As increasing the value reduces the frequency of
generation, it may cause you to miss shorter-lived transient spikes in some values. That's the tradeoff you have to
evaluate. But having a value smaller than the time taken to process the publications might result in a never-ending
scrape. The publication-processing portion of the scrape can be seen in a debug log.

## Reducing subscriptions made to queue manager
Reducing the total number of subscriptions made will reduce the data that needs to be processed. But at the cost of
missing some metrics that you might find useful. See also the section in the [README](README.md) file about using
durable subscriptions.

* You can disable all use of published resource metrics, and rely on the `DISPLAY xxSTATUS` responses. This clearly
  reduces the data, but you lose out on many useful metrics. It is essentially how we monitor z/OS queue managers as
  they do not have the publication model for metrics. But if you want this approach, set the `global.usePublications`
  configuration option to `false`

* You can reduce the total number of subscriptions made for queue metrics. The `filters.queueSubscriptionSelector` list
  defines the sets of topics that you might be interested in. The complete set - for now - is
  [OPENCLOSE, INQSET, PUT, GET, GENERAL]. In many cases, only the last three of these may be of interest. The smaller
  set reduces the number of publications per queue. Within each set, multiple metrics are created but there is no way to
  report on only a subset of the metrics in each set.

* You can choose to not subscribe to any queue metrics, but still subscribe to metrics for other resources such as the
  queue manager and Native HA by setting the filter to `NONE`. If you do this, then many queue metrics become
  unavailable. However, the current queue depth will still be available as it can also be determined from the
  `DISPLAY QSTATUS` response.

## Reducing the number of monitored objects and status requests
Each object type (queues, channels etc) has a block in the collector configuration that names which objects should be
monitored. While both positive and negative wildcards can be used in these blocks, it is probably most efficient to use
only positive wildcards. That allows the `DISPLAY xxSTATUS` requests to pass the wildcards directly into the queue
manager commands; if there are any negative patterns, the collector has to work out which objects match the pattern, and
then inquire for the remainder individually.

## Other configuration options
The `global.pollInterval` and `global.rediscoverInterval` options may help to further reduce inquiries.

The first of these controls how frequently the `DISPLAY xxSTATUS` commands are used, assuming the
`global.useObjectStatus` is `true`. In some circumstances, you might not want all of the responses as regularly as the
published metrics are handled. The queue and queue manager status is always collected during this phase regardless of
the `global.useObjectStatus` setting.

The second attribute controls how frequently the collector reassesses the list of objects to be monitored, and their
more stable attributes. For example, the `DESCRIPTION` or `MAXDEPTH` settings on a queue. If you have a large number of
queues that do not change frequently, then you might want to increase the rediscovery attribute. The default is 1 hour.
The tradeoff here is that newly-defined queues may not have any metrics reported until this interval expires.

The messages created by the queue manager ought to be non-persistent. There is no real value in preserving them across a
restart. Check that the configured reply queues, whether model or local queues, have `DEFPSIST(NO)`.

## Dividing the workload
One further approach that you might like to consider, though I wouldn't usually recommend it, is to have two or more
collectors running against the same queue manager. And then configure different sets of queues to be monitored. So a
collector listening on port 9157 might manage queues A*-M*, while another collector on port 9158 monitors queues N*-Z*.
You would likely need additional configuration to reduce duplication of other components, for example by using the
`jobname` or `instance` as a filter element on dashboard queries, but it might be one way to reduce the time taken for a
single scrape.

## Very slow queue managers
The collectors wait for a short time for each response to a status request. If the timeout expires with no expected
message appearing, then an error is reported. Some queue managers - particuarly when hosted in cloud services - have
appeared to "stall" for a period. Even though they are not especially busy, the response messages have not appeared in
time. The default wait of 3 seconds can be tuned using the `connection.waitInterval` option.

For all collectors _except_ Prometheus, a small number of these timeout errors are permitted consecutively. The failure
count is reset after a successful collection. See _pkg/errors/errors.go_ for details. The Prometheus collector has an
automatic reconnect option after failures, so does not currently use this strategy.

## Prometheus configuration
There are some options that can be applied in the Prometheus configuration for reducing the number of metrics stored.
These do not affect the collection from the queue manager, but Prometheus can apply filters during the collection phase
so that only a subset is actually written to the database. This can be particularly relevant if you are then sending the
data onwards to something like Grafana Cloud. The `metric_relabel_configs` section of Prometheus configuration seems to
be the key area.

See
[here](https://grafana.com/docs/grafana-cloud/cost-management-and-billing/reduce-costs/metrics-costs/client-side-filtering/)
and [here](https://www.robustperception.io/dropping-metrics-at-scrape-time-with-prometheus/) for more details.
