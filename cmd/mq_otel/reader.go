package main

/*
  Copyright (c) IBM Corporation 2024

  Licensed under the Apache License, Version 2.0 (the "License");
  you may not use this file except in compliance with the License.
  You may obtain a copy of the License at

  http://www.apache.org/licenses/LICENSE-2.0

  Unless required by applicable law or agreed to in writing, software
  distributed under the License is distributed on an "AS IS" BASIS,
  WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
  See the License for the specific

   Contributors:
     Mark Taylor - Initial Contribution
*/

/*
This file pushes collected data to the OpenTelemetry system over GRPC.
The GetMetrics() function is the key operation
invoked at the configured intervals, causing us to read available publications
and status responses.
*/

import (
	"context"
	"strings"
	"time"

	ibmmq "github.com/ibm-messaging/mq-golang/v5/ibmmq"
	mqmetric "github.com/ibm-messaging/mq-golang/v5/mqmetric"
	errors "github.com/ibm-messaging/mq-metric-samples/v5/pkg/errors"

	attribute "go.opentelemetry.io/otel/attribute"
	metric "go.opentelemetry.io/otel/metric"

	log "github.com/sirupsen/logrus"
)

// Each Gauge is handled as a list (array) of structures, to deal with
// multiple objects reporting the same metric eg channel_messages. We don't
// need an equivalent for Counters because those are reported synchronously
// The OTel specification does define synchronous Gauges, but those have not
// yet made their way into the Go libraries as they are currently marked "experimental".
// See https://github.com/open-telemetry/opentelemetry-go/issues/3984
// and https://github.com/open-telemetry/opentelemetry-specification/blob/main/specification/metrics/api.md#gauge
type gaugeStruct struct {
	value float64
	tags  attribute.Set
}

var (
	first = true

	platformString     = ""
	lastPoll           = time.Now()
	lastQueueDiscovery time.Time

	counterMap map[string]metric.Float64Counter = make(map[string]metric.Float64Counter)
	gaugeMap   map[string][]gaugeStruct         = make(map[string][]gaugeStruct)
)

// The value is passed in as a float64 (to match other exporters/collectors in this repo).
// Can we set the timestamp for when it was collected rather than "now"? Does not appear so but I've kept it
// in these functions for now.

// These short functions extract fields from the two different styles of metric we collect, and pass them down to the common piece
func addMetricE(Meter metric.Meter, series string, elem *mqmetric.MonElement, value interface{}, tags map[string]string, readTime time.Time) {
	delta := false
	if elem.Datatype == ibmmq.MQIAMO_MONITOR_DELTA {
		delta = true
	}
	addMetric(meter, series, elem.MetricName, elem.Description, delta, value, tags, readTime)
}

func addMetricA(Meter metric.Meter, series string, attr *mqmetric.StatusAttribute, value interface{}, tags map[string]string, readTime time.Time) {
	addMetric(meter, series, attr.MetricName, attr.Description, attr.Delta, value, tags, readTime)
}

// This is where we add the metric's value to the set that will be reported on this pass.
// Two different sorts of metric report are used:
// - a Counter is for those that can be accumulated over time (the delta flag is enabled)
// - a Gauge is used for "absolute" numbers eg number of current connections
func addMetric(meter metric.Meter, series string, metricName string, desc string, cumul bool, value interface{}, tags map[string]string, readTime time.Time) {
	var err error
	var pt metric.Float64Counter
	var guAr []gaugeStruct

	metricUnit := "1"
	ok := true

	// Some metrics look a bit silly with the name by default coming out looking like queue_queue_depth.
	// So we strip the 2nd "queue".
	if series == "queue" && strings.HasPrefix(metricName, "queue_") {
		metricName = metricName[len("queue_"):]
	}

	// Optionally force all metrics to be reported as Gauges for compatibility with the mq_prometheus collector in this repo
	if config.overrideCType {
		cumul = false
	}

	counterKey := "ibmmq." + series + "." + metricName
	if cumul {
		if pt, ok = counterMap[counterKey]; !ok {
			pt, err = meter.Float64Counter(counterKey, metric.WithDescription(desc), metric.WithUnit(metricUnit))

			if err == nil {
				log.Debugf("Created counter for %s", counterKey)
				counterMap[counterKey] = pt

			} else {
				log.Debugf("Error creating counter for %s: %v", counterKey, err)
				return
			}
		}
	} else {
		if guAr, ok = gaugeMap[counterKey]; !ok {
			// The callback function needs to be passed some correlator to say which
			// metric it's for. So we're going to end up with a long list of very similar
			// functions.
			k := counterKey
			_, err = meter.Float64ObservableGauge(counterKey, metric.WithDescription(desc),
				metric.WithUnit(metricUnit),
				metric.WithFloat64Callback(func(ctx context.Context, o metric.Float64Observer) error {
					return observe(ctx, o, k)
				}))

			if err == nil {
				log.Debugf("Created gauge   for %s", counterKey)
				guAr = nil
				gaugeMap[counterKey] = guAr

			} else {
				log.Debugf("Error creating gauge for %s: %v", counterKey, err)
				return
			}
		}
	}

	attrs := make([]attribute.KeyValue, 0, len(tags))
	for ak, av := range tags {
		attrs = append(attrs, attribute.String(ak, av))
	}

	tagSet := attribute.NewSet(attrs...)

	// Counters can be added immediately to the metric list; Gauges can only be added by a Callback function
	// so we stash the value as an array element.
	v := value.(float64)
	if cumul {
		//log.Debugf("Cumul: Metric:%s Val:%f Tags:%v", counterKey, v, tagSet)
		pt.Add(ctx, v, metric.WithAttributeSet(tagSet))
	} else {
		//log.Debugf("Stash: Metric:%s Val:%f Tags:%v", counterKey, v, tagSet)
		m := gaugeStruct{value: v, tags: tagSet}
		guAr = append(guAr, m)
		gaugeMap[counterKey] = guAr
	}

	// deepDebug("Counter: %+v", pt)

	return
}

// This function is called during the Collect phase as a callback for all gauges. Each
// metric might have multiple values, one for each object reporting the metric. They have
// been stashed in arrays which in turn are in a map keyed by the metric name
func observe(_ context.Context, o metric.Float64Observer, counterKey string) error {
	if guAr, ok := gaugeMap[counterKey]; ok {
		for i := 0; i < len(guAr); i++ {
			g := guAr[i]
			//log.Debugf("Observe: Metric:%s Val:%f Tags:%v", counterKey, g.value, g.tags)
			o.Observe(g.value, metric.WithAttributeSet(g.tags))
		}
	}
	return nil
}

/*
This function is called by the main routine at regular intervals to provide current data
*/
func GetMetrics(ctx context.Context, meter metric.Meter) error {

	var err error
	var series string
	var t time.Time

	log.Debugf("IBMMQ OpenTelemetry collection started")
	collectStartTime := time.Now()

	if platformString == "" {
		platformString = strings.Replace(ibmmq.MQItoString("PL", int(mqmetric.GetPlatform())), "MQPL_", "", -1)
	}

	// Clear out everything we know so far. In particular, replace
	// the maps of values for each object so the collection starts
	// clean.
	for _, cl := range mqmetric.GetPublishedMetrics("").Classes {
		for _, ty := range cl.Types {
			for _, elem := range ty.Elements {
				elem.Values = make(map[string]int64)
			}
		}
	}

	for gaugeName, _ := range gaugeMap {
		gaugeMap[gaugeName] = nil
	}

	// Process all the publications that have arrived
	err = mqmetric.ProcessPublications()
	if err != nil {
		log.Fatalf("Error processing publications: %v", err)
	} else {
		log.Debugf("Processed all publications")
	}

	// Do we need to poll for object status on this iteration
	pollStatus := false
	thisPoll := time.Now()
	elapsed := thisPoll.Sub(lastPoll)
	if elapsed >= config.cf.PollIntervalDuration || first {
		log.Debugf("Polling for object status")
		lastPoll = thisPoll
		pollStatus = true
	} else {
		log.Debugf("Skipping poll for object status")
	}

	pollError := err

	// If there has been sufficient interval since the last explicit poll for
	// status, then do that collection too.
	if pollStatus {
		if config.cf.CC.UseStatus {
			err = mqmetric.CollectChannelStatus(config.cf.MonitoredChannels)
			if err != nil {
				log.Errorf("Error collecting channel status: %v", err)
				pollError = err
			} else {
				log.Debugf("Collected all channel status")
			}
			err = mqmetric.CollectTopicStatus(config.cf.MonitoredTopics)
			if err != nil {
				log.Errorf("Error collecting topic status: %v", err)
				pollError = err
			} else {
				log.Debugf("Collected all topic status")
			}
			err = mqmetric.CollectSubStatus(config.cf.MonitoredSubscriptions)
			if err != nil {
				log.Errorf("Error collecting subscription status: %v", err)
				pollError = err
			} else {
				log.Debugf("Collected all subscription status")
			}

			err = mqmetric.CollectQueueStatus(config.cf.MonitoredQueues)
			if err != nil {
				log.Errorf("Error collecting queue status: %v", err)
				pollError = err
			} else {
				log.Debugf("Collected all queue status")
			}

			err = mqmetric.CollectClusterStatus()
			if err != nil {
				log.Errorf("Error collecting cluster status: %v", err)
				pollError = err
			} else {
				log.Debugf("Collected all cluster status")
			}

			if mqmetric.GetPlatform() == ibmmq.MQPL_ZOS {
				err = mqmetric.CollectUsageStatus()
				if err != nil {
					log.Errorf("Error collecting bufferpool/pageset status: %v", err)
					pollError = err
				} else {
					log.Debugf("Collected all buffer pool/pageset status")
				}
			} else {
				if config.cf.MonitoredAMQPChannels != "" {
					err = mqmetric.CollectAMQPChannelStatus(config.cf.MonitoredAMQPChannels)
					if err != nil {
						log.Errorf("Error collecting AMQP status: %v", err)
						pollError = err
					} else {
						log.Debugf("Collected all AMQP status")
					}
				}

				if config.cf.MonitoredMQTTChannels != "" {
					err = mqmetric.CollectMQTTChannelStatus(config.cf.MonitoredMQTTChannels)
					if err != nil {
						log.Errorf("Error collecting MQTT status: %v", err)
						pollError = err
					} else {
						log.Debugf("Collected all MQTT status")
					}
				}
			}

			err = mqmetric.CollectQueueManagerStatus()
			if err != nil {
				log.Errorf("Error collecting queue manager status: %v", err)
				pollError = err
			} else {
				log.Debugf("Collected all queue manager status")
			}
			err = pollError
		}

		errors.HandleStatus(err)

		thisDiscovery := time.Now()
		elapsed = thisDiscovery.Sub(lastQueueDiscovery)
		if config.cf.RediscoverDuration > 0 {
			if elapsed >= config.cf.RediscoverDuration {
				log.Debugf("Doing queue rediscovery")
				/*err =*/ mqmetric.RediscoverAndSubscribe(discoverConfig)
				lastQueueDiscovery = thisDiscovery
				/*err =*/ mqmetric.RediscoverAttributes(ibmmq.MQOT_CHANNEL, config.cf.MonitoredChannels)
				if mqmetric.GetPlatform() != ibmmq.MQPL_ZOS {
					err = mqmetric.RediscoverAttributes(mqmetric.OT_CHANNEL_AMQP, config.cf.MonitoredAMQPChannels)
					err = mqmetric.RediscoverAttributes(mqmetric.OT_CHANNEL_MQTT, config.cf.MonitoredMQTTChannels)
				}

			}
		}

		// Have now processed all of the publications, and all the MQ-owned
		// value fields and maps have been updated.
		//
		// Now need to set all of the real items with the correct values
		if first {
			// Always ignore the first loop through as there might
			// be accumulated stuff from a while ago, and lead to
			// a misleading range on graphs.
			first = false
		} else {
			t = time.Now()

			// Start with a metric that shows how many publications were processed by this collection
			series = "qmgr"
			tags := map[string]string{
				"qmgr":     config.cf.QMgrName,
				"platform": platformString,
			}
			addMetaLabels(tags)

			log.Debugf("Processed %d publications", mqmetric.GetProcessPublicationCount())
			addMetric(meter, series, "exporter_publications", "Publications Processed", false, float64(mqmetric.GetProcessPublicationCount()), tags, t)

			// Dump the published resource metrics
			for _, cl := range mqmetric.GetPublishedMetrics("").Classes {
				for _, ty := range cl.Types {
					for _, elem := range ty.Elements {
						for key, value := range elem.Values {
							f := mqmetric.Normalise(elem, key, value)
							tags = map[string]string{
								"qmgr":     config.cf.QMgrName,
								"platform": platformString,
							}

							series = "qmgr"
							if key == mqmetric.QMgrMapKey {
								tags["description"] = mqmetric.GetObjectDescription("", ibmmq.MQOT_Q_MGR)
								hostname := mqmetric.GetQueueManagerAttribute(config.cf.QMgrName, ibmmq.MQCACF_HOST_NAME)
								if hostname != mqmetric.DUMMY_STRING {
									tags["hostname"] = hostname
								}
							} else if strings.HasPrefix(key, mqmetric.NativeHAKeyPrefix) {
								series = "nha"
								tags["nha"] = strings.Replace(key, mqmetric.NativeHAKeyPrefix, "", -1)
							} else {
								tags["queue"] = key
								series = "queue"
								usage := ""
								if usageAttr, ok := mqmetric.GetObjectStatus("", mqmetric.OT_Q).Attributes[mqmetric.ATTR_Q_USAGE].Values[key]; ok {
									if usageAttr.ValueInt64 == 1 {
										usage = "XMITQ"
									} else {
										usage = "NORMAL"
									}
								}

								tags["usage"] = usage
								tags["description"] = mqmetric.GetObjectDescription(key, ibmmq.MQOT_Q)
								tags["cluster"] = mqmetric.GetQueueAttribute(key, ibmmq.MQCA_CLUSTER_NAME)
							}

							addMetaLabels(tags)
							addMetricE(meter, series, elem, f, tags, t)
						}
					}
				}
			}

			// Now report all of those collected by DIS xxSTATUS commands etc.
			series = "queue"
			for _, attr := range mqmetric.GetObjectStatus("", mqmetric.OT_Q).Attributes {
				for key, value := range attr.Values {
					if value.IsInt64 {

						qName := mqmetric.GetObjectStatus("", mqmetric.OT_Q).Attributes[mqmetric.ATTR_Q_NAME].Values[key].ValueString

						tags := map[string]string{
							"qmgr": config.cf.QMgrName,
						}
						tags["queue"] = qName
						tags["platform"] = platformString
						usage := ""
						if usageAttr, ok := mqmetric.GetObjectStatus("", mqmetric.OT_Q).Attributes[mqmetric.ATTR_Q_USAGE].Values[key]; ok {
							if usageAttr.ValueInt64 == 1 {
								usage = "XMITQ"
							} else {
								usage = "NORMAL"
							}
						}

						tags["usage"] = usage
						tags["description"] = mqmetric.GetObjectDescription(key, ibmmq.MQOT_Q)
						tags["cluster"] = mqmetric.GetQueueAttribute(key, ibmmq.MQCA_CLUSTER_NAME)
						addMetaLabels(tags)

						f := mqmetric.QueueNormalise(attr, value.ValueInt64)
						addMetricA(meter, series, attr, f, tags, t)
					}
				}
			}

			series = "channel"
			for _, attr := range mqmetric.GetObjectStatus("", mqmetric.OT_CHANNEL).Attributes {
				for key, value := range attr.Values {
					if value.IsInt64 {

						chlType := int(mqmetric.GetObjectStatus("", mqmetric.OT_CHANNEL).Attributes[mqmetric.ATTR_CHL_TYPE].Values[key].ValueInt64)
						chlTypeString := strings.Replace(ibmmq.MQItoString("CHT", chlType), "MQCHT_", "", -1)
						// Not every channel status report has the RQMNAME attribute (eg SVRCONNs)
						rqmName := ""
						if rqmNameAttr, ok := mqmetric.GetObjectStatus("", mqmetric.OT_CHANNEL).Attributes[mqmetric.ATTR_CHL_RQMNAME].Values[key]; ok {
							rqmName = rqmNameAttr.ValueString
						}

						chlName := mqmetric.GetObjectStatus("", mqmetric.OT_CHANNEL).Attributes[mqmetric.ATTR_CHL_NAME].Values[key].ValueString
						connName := mqmetric.GetObjectStatus("", mqmetric.OT_CHANNEL).Attributes[mqmetric.ATTR_CHL_CONNNAME].Values[key].ValueString
						jobName := mqmetric.GetObjectStatus("", mqmetric.OT_CHANNEL).Attributes[mqmetric.ATTR_CHL_JOBNAME].Values[key].ValueString
						cipherSpec := mqmetric.DUMMY_STRING
						if cipherSpecAttr, ok := mqmetric.GetObjectStatus("", mqmetric.OT_CHANNEL).Attributes[mqmetric.ATTR_CHL_SSLCIPH].Values[key]; ok {
							cipherSpec = cipherSpecAttr.ValueString
						}

						tags := map[string]string{
							"qmgr": config.cf.QMgrName,
						}
						tags["channel"] = chlName
						tags["platform"] = platformString
						tags[mqmetric.ATTR_CHL_TYPE] = strings.TrimSpace(chlTypeString)
						tags[mqmetric.ATTR_CHL_RQMNAME] = strings.TrimSpace(rqmName)
						tags[mqmetric.ATTR_CHL_CONNNAME] = strings.TrimSpace(connName)
						tags[mqmetric.ATTR_CHL_JOBNAME] = strings.TrimSpace(jobName)
						tags[mqmetric.ATTR_CHL_SSLCIPH] = strings.TrimSpace(cipherSpec)
						addMetaLabels(tags)

						f := mqmetric.ChannelNormalise(attr, value.ValueInt64)
						addMetricA(meter, series, attr, f, tags, t)
					}
				}
			}

			series = "topic"
			for _, attr := range mqmetric.GetObjectStatus("", mqmetric.OT_TOPIC).Attributes {
				for key, value := range attr.Values {
					if value.IsInt64 {

						topicString := mqmetric.GetObjectStatus("", mqmetric.OT_TOPIC).Attributes[mqmetric.ATTR_TOPIC_STRING].Values[key].ValueString
						topicStatusType := mqmetric.GetObjectStatus("", mqmetric.OT_TOPIC).Attributes[mqmetric.ATTR_TOPIC_STATUS_TYPE].Values[key].ValueString

						tags := map[string]string{
							"qmgr": config.cf.QMgrName,
						}
						tags["topic"] = topicString
						tags["platform"] = platformString
						tags["type"] = topicStatusType
						addMetaLabels(tags)

						f := mqmetric.TopicNormalise(attr, value.ValueInt64)
						addMetricA(meter, series, attr, f, tags, t)

					}
				}
			}

			series = "subscription"
			for _, attr := range mqmetric.GetObjectStatus("", mqmetric.OT_SUB).Attributes {
				for key, value := range attr.Values {
					if value.IsInt64 {
						subId := mqmetric.GetObjectStatus("", mqmetric.OT_SUB).Attributes[mqmetric.ATTR_SUB_ID].Values[key].ValueString
						subName := mqmetric.GetObjectStatus("", mqmetric.OT_SUB).Attributes[mqmetric.ATTR_SUB_NAME].Values[key].ValueString
						subType := int(mqmetric.GetObjectStatus("", mqmetric.OT_SUB).Attributes[mqmetric.ATTR_SUB_TYPE].Values[key].ValueInt64)
						subTypeString := strings.Replace(ibmmq.MQItoString("SUBTYPE", subType), "MQSUBTYPE_", "", -1)
						topicString := mqmetric.GetObjectStatus("", mqmetric.OT_SUB).Attributes[mqmetric.ATTR_SUB_TOPIC_STRING].Values[key].ValueString

						tags := map[string]string{
							"qmgr": config.cf.QMgrName,
						}

						tags["platform"] = platformString
						tags["type"] = subTypeString
						tags["subid"] = subId
						tags["subscription"] = subName
						tags["topic"] = topicString
						addMetaLabels(tags)

						f := mqmetric.SubNormalise(attr, value.ValueInt64)
						addMetricA(meter, series, attr, f, tags, t)

					}
				}
			}

			series = "cluster"
			for _, attr := range mqmetric.GetObjectStatus("", mqmetric.OT_CLUSTER).Attributes {
				for key, value := range attr.Values {
					if value.IsInt64 {
						clusterName := mqmetric.GetObjectStatus("", mqmetric.OT_CLUSTER).Attributes[mqmetric.ATTR_CLUSTER_NAME].Values[key].ValueString
						qmType := mqmetric.GetObjectStatus("", mqmetric.OT_CLUSTER).Attributes[mqmetric.ATTR_CLUSTER_QMTYPE].Values[key].ValueInt64

						qmTypeString := "PARTIAL"
						if qmType == int64(ibmmq.MQQMT_REPOSITORY) {
							qmTypeString = "FULL"
						}

						tags := map[string]string{
							"qmgr":     config.cf.QMgrName,
							"cluster":  clusterName,
							"qmtype":   qmTypeString,
							"platform": platformString,
						}
						addMetaLabels(tags)

						f := mqmetric.ClusterNormalise(attr, value.ValueInt64)
						addMetricA(meter, series, attr, f, tags, t)

					}
				}
			}

			series = "qmgr"
			for _, attr := range mqmetric.QueueManagerStatus.Attributes {
				for _, value := range attr.Values {
					if value.IsInt64 {

						qMgrName := strings.TrimSpace(config.cf.QMgrName)

						tags := map[string]string{
							"qmgr":        qMgrName,
							"platform":    platformString,
							"description": mqmetric.GetObjectDescription("", ibmmq.MQOT_Q_MGR),
						}
						hostname := mqmetric.GetQueueManagerAttribute(config.cf.QMgrName, ibmmq.MQCACF_HOST_NAME)
						if hostname != mqmetric.DUMMY_STRING {
							tags["hostname"] = hostname
						}
						addMetaLabels(tags)

						f := mqmetric.QueueManagerNormalise(attr, value.ValueInt64)
						addMetricA(meter, series, attr, f, tags, t)

					}
				}
			}

			if mqmetric.GetPlatform() == ibmmq.MQPL_ZOS {
				series = "bufferpool"
				for _, attr := range mqmetric.GetObjectStatus("", mqmetric.OT_BP).Attributes {
					for key, value := range attr.Values {
						bpId := mqmetric.GetObjectStatus("", mqmetric.OT_BP).Attributes[mqmetric.ATTR_BP_ID].Values[key].ValueString
						bpLocation := mqmetric.GetObjectStatus("", mqmetric.OT_BP).Attributes[mqmetric.ATTR_BP_LOCATION].Values[key].ValueString
						bpClass := mqmetric.GetObjectStatus("", mqmetric.OT_BP).Attributes[mqmetric.ATTR_BP_CLASS].Values[key].ValueString
						if value.IsInt64 && !attr.Pseudo {
							qMgrName := strings.TrimSpace(config.cf.QMgrName)

							tags := map[string]string{
								"qmgr":     qMgrName,
								"platform": platformString,
							}
							tags["bufferpool"] = bpId
							tags["location"] = bpLocation
							tags["pageclass"] = bpClass
							addMetaLabels(tags)

							f := mqmetric.UsageNormalise(attr, value.ValueInt64)
							addMetricA(meter, series, attr, f, tags, t)

						}
					}
				}

				series = "pageset"
				for _, attr := range mqmetric.GetObjectStatus("", mqmetric.OT_PS).Attributes {
					for key, value := range attr.Values {
						qMgrName := strings.TrimSpace(config.cf.QMgrName)
						psId := mqmetric.GetObjectStatus("", mqmetric.OT_PS).Attributes[mqmetric.ATTR_PS_ID].Values[key].ValueString
						bpId := mqmetric.GetObjectStatus("", mqmetric.OT_PS).Attributes[mqmetric.ATTR_PS_BPID].Values[key].ValueString
						if value.IsInt64 && !attr.Pseudo {
							tags := map[string]string{
								"qmgr":     qMgrName,
								"platform": platformString,
							}
							tags["pageset"] = psId
							tags["bufferpool"] = bpId
							addMetaLabels(tags)

							f := mqmetric.UsageNormalise(attr, value.ValueInt64)
							addMetricA(meter, series, attr, f, tags, t)

						}
					}
				}
			} else {
				series = "amqp"
				for _, attr := range mqmetric.GetObjectStatus("", mqmetric.OT_CHANNEL_AMQP).Attributes {
					qMgrName := strings.TrimSpace(config.cf.QMgrName)

					for key, value := range attr.Values {
						chlName := mqmetric.GetObjectStatus("", mqmetric.OT_CHANNEL_AMQP).Attributes[mqmetric.ATTR_CHL_NAME].Values[key].ValueString
						clientId := mqmetric.GetObjectStatus("", mqmetric.OT_CHANNEL_AMQP).Attributes[mqmetric.ATTR_CHL_AMQP_CLIENT_ID].Values[key].ValueString
						connName := mqmetric.GetObjectStatus("", mqmetric.OT_CHANNEL_AMQP).Attributes[mqmetric.ATTR_CHL_CONNNAME].Values[key].ValueString
						if value.IsInt64 && !attr.Pseudo {
							tags := map[string]string{
								"qmgr":     qMgrName,
								"platform": platformString,
							}
							tags["channel"] = chlName
							tags["description"] = mqmetric.GetObjectDescription(chlName, mqmetric.OT_CHANNEL_AMQP)
							tags[mqmetric.ATTR_CHL_CONNNAME] = strings.TrimSpace(connName)
							tags[mqmetric.ATTR_CHL_AMQP_CLIENT_ID] = clientId
							addMetaLabels(tags)

							f := mqmetric.ChannelNormalise(attr, value.ValueInt64)
							addMetricA(meter, series, attr, f, tags, t)
						}
					}
				}

				series = "mqtt"
				for _, attr := range mqmetric.GetObjectStatus("", mqmetric.OT_CHANNEL_MQTT).Attributes {
					qMgrName := strings.TrimSpace(config.cf.QMgrName)

					for key, value := range attr.Values {
						chlName := mqmetric.GetObjectStatus("", mqmetric.OT_CHANNEL_MQTT).Attributes[mqmetric.ATTR_CHL_NAME].Values[key].ValueString
						clientId := mqmetric.GetObjectStatus("", mqmetric.OT_CHANNEL_MQTT).Attributes[mqmetric.ATTR_CHL_MQTT_CLIENT_ID].Values[key].ValueString
						connName := mqmetric.GetObjectStatus("", mqmetric.OT_CHANNEL_MQTT).Attributes[mqmetric.ATTR_CHL_CONNNAME].Values[key].ValueString
						if value.IsInt64 && !attr.Pseudo {
							tags := map[string]string{
								"qmgr":     qMgrName,
								"platform": platformString,
							}
							tags["channel"] = chlName
							tags["description"] = mqmetric.GetObjectDescription(chlName, mqmetric.OT_CHANNEL_MQTT)
							tags[mqmetric.ATTR_CHL_CONNNAME] = strings.TrimSpace(connName)
							tags[mqmetric.ATTR_CHL_MQTT_CLIENT_ID] = clientId
							addMetaLabels(tags)

							f := mqmetric.ChannelNormalise(attr, value.ValueInt64)
							addMetricA(meter, series, attr, f, tags, t)
						}
					}
				}
			}
		}
	}

	collectStopTime := time.Now()
	elapsedSecs := int64(collectStopTime.Sub(collectStartTime).Seconds())
	log.Debugf("Collection time = %d secs", elapsedSecs)

	series = "qmgr"
	tags := map[string]string{
		"qmgr":     config.cf.QMgrName,
		"platform": platformString,
	}
	addMetaLabels(tags)
	addMetric(meter, series, "exporter_collection_time", "How long last collection took", false, float64(elapsedSecs), tags, t)

	return err

}

func addMetaLabels(tags map[string]string) {
	if len(config.cf.MetadataTagsArray) > 0 {
		for i := 0; i < len(config.cf.MetadataTagsArray); i++ {
			tags[config.cf.MetadataTagsArray[i]] = config.cf.MetadataValuesArray[i]
		}
	}
}
