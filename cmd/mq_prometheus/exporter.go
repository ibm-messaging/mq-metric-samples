package main

/*
  Copyright (c) IBM Corporation 2016, 2025

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
This file provides the main link between the MQ monitoring collection, and
the Prometheus request for data. The Collect() function is the key operation
invoked at the scrape intervals, causing us to read available publications
and update the various Gauges.
*/

import (
	"os"
	"strings"
	"time"

	"github.com/ibm-messaging/mq-golang/v5/ibmmq"
	"github.com/ibm-messaging/mq-golang/v5/mqmetric"
	"github.com/prometheus/client_golang/prometheus"
	log "github.com/sirupsen/logrus"
)

type exporter struct {
	metrics       *mqmetric.AllMetrics
	chlStatus     *mqmetric.StatusSet
	qStatus       *mqmetric.StatusSet
	topicStatus   *mqmetric.StatusSet
	subStatus     *mqmetric.StatusSet
	qMgrStatus    *mqmetric.StatusSet
	usageBpStatus *mqmetric.StatusSet
	usagePsStatus *mqmetric.StatusSet
	clusterStatus *mqmetric.StatusSet
	amqpStatus    *mqmetric.StatusSet
	mqttStatus    *mqmetric.StatusSet
}

func newExporter() *exporter {
	return &exporter{
		metrics:       mqmetric.GetPublishedMetrics(""),
		chlStatus:     mqmetric.GetObjectStatus("", mqmetric.OT_CHANNEL),
		qStatus:       mqmetric.GetObjectStatus("", mqmetric.OT_Q),
		topicStatus:   mqmetric.GetObjectStatus("", mqmetric.OT_TOPIC),
		subStatus:     mqmetric.GetObjectStatus("", mqmetric.OT_SUB),
		qMgrStatus:    mqmetric.GetObjectStatus("", mqmetric.OT_Q_MGR),
		usageBpStatus: mqmetric.GetObjectStatus("", mqmetric.OT_BP),
		usagePsStatus: mqmetric.GetObjectStatus("", mqmetric.OT_PS),
		clusterStatus: mqmetric.GetObjectStatus("", mqmetric.OT_CLUSTER),
		amqpStatus:    mqmetric.GetObjectStatus("", mqmetric.OT_CHANNEL_AMQP),
		mqttStatus:    mqmetric.GetObjectStatus("", mqmetric.OT_CHANNEL_MQTT),
	}
}

const (
	defaultScrapeTimeout = 15 // Prometheus default scrape_timeout is 15s
)

// Container for metrics of either kind
type MQVec struct {
	n string
	c *prometheus.CounterVec
	g *prometheus.GaugeVec
}

var (
	ruaVecMap           = make(map[string]*MQVec) // Metrics collected via the publication route like amqsrua
	channelStatusVecMap = make(map[string]*MQVec)
	qStatusVecMap       = make(map[string]*MQVec)
	topicStatusVecMap   = make(map[string]*MQVec)
	subStatusVecMap     = make(map[string]*MQVec)
	qMgrStatusVecMap    = make(map[string]*MQVec)
	usageBpStatusVecMap = make(map[string]*MQVec)
	usagePsStatusVecMap = make(map[string]*MQVec)
	clusterStatusVecMap = make(map[string]*MQVec)
	amqpStatusVecMap    = make(map[string]*MQVec)
	mqttStatusVecMap    = make(map[string]*MQVec)

	lastPoll           = time.Now()
	lastQueueDiscovery time.Time
	platformString     string
	// counter               = 0
	scrapeWarningIssued   = false
	scrapeWarningPossible = false
	pubCountDesc          *prometheus.Desc
	collectionTimeDesc    *prometheus.Desc

	supportsHostnameLabelVal *bool
	lastHostname             = mqmetric.DUMMY_STRING

	unittestLoops    = 0
	unittestMaxLoops = 0
)

/*
Describe is called by Prometheus on startup of this monitor. It needs to tell
the caller about all of the available metrics. It is also called during "unregister"
after a reconnection to a failed qmgr has succeeded.
*/
func (e *exporter) Describe(ch chan<- *prometheus.Desc) {

	platformString = strings.Replace(ibmmq.MQItoString("PL", int(mqmetric.GetPlatform())), "MQPL_", "", -1)

	// Seeing this twice in succession in the logs is a bit odd. So we'll set things up that
	// the invocation is not reported during an Unregister operation.
	if !isCollectorSilent() {
		log.Infof("IBMMQ Describe started")
		log.Infof("Platform is %s", platformString)
	}

	for _, cl := range e.metrics.Classes {
		for _, ty := range cl.Types {
			for _, elem := range ty.Elements {
				ruaVecMap[makeKey(elem)].Describe(ch)
			}
		}
	}

	for _, attr := range e.chlStatus.Attributes {
		channelStatusVecMap[attr.MetricName].Describe(ch)
	}
	for _, attr := range e.qStatus.Attributes {
		qStatusVecMap[attr.MetricName].Describe(ch)
	}
	for _, attr := range e.topicStatus.Attributes {
		topicStatusVecMap[attr.MetricName].Describe(ch)
	}
	for _, attr := range e.subStatus.Attributes {
		subStatusVecMap[attr.MetricName].Describe(ch)
	}

	for _, attr := range e.clusterStatus.Attributes {
		clusterStatusVecMap[attr.MetricName].Describe(ch)
	}

	// DISPLAY QMSTATUS is not supported on z/OS
	// but we do extract a couple of MQINQable attributes
	for _, attr := range e.qMgrStatus.Attributes {
		qMgrStatusVecMap[attr.MetricName].Describe(ch)
	}

	// The BufferPool and PageSet stuff is only for z/OS
	if mqmetric.GetPlatform() == ibmmq.MQPL_ZOS {
		for _, attr := range e.usageBpStatus.Attributes {
			usageBpStatusVecMap[attr.MetricName].Describe(ch)
		}
		for _, attr := range e.usagePsStatus.Attributes {
			usagePsStatusVecMap[attr.MetricName].Describe(ch)
		}
	} else {
		// While AMQP and MQTT are Distributed only
		for _, attr := range e.amqpStatus.Attributes {
			amqpStatusVecMap[attr.MetricName].Describe(ch)
		}
		for _, attr := range e.mqttStatus.Attributes {
			mqttStatusVecMap[attr.MetricName].Describe(ch)
		}
	}
}

/*
I put the actual collection callback in this function to make it easy to
add timing/debug around it
*/
func (v *MQVec) CollectWrap(ch chan<- prometheus.Metric) {
	v.Collect(ch)
}

func (v *MQVec) Describe(ch chan<- *prometheus.Desc) {
	if v.g != nil {
		v.g.Describe(ch)
	} else {
		v.c.Describe(ch)
	}
}

func (v *MQVec) Collect(ch chan<- prometheus.Metric) {
	if v.g != nil {
		v.g.Collect(ch)
	} else {
		v.c.Collect(ch)
	}
}

func (v *MQVec) Reset() {
	if v.g != nil {
		v.g.Reset()
	} else {
		v.c.Reset()
	}
}

func (v *MQVec) addMetric(labels prometheus.Labels, f float64) {
	if v.g != nil {
		//log.Debugf("SET: %s %v %f", v.n, labels, f)
		v.g.With(labels).Set(f)
	} else {
		//log.Debugf("ADD: %s %v %f", v.n, labels, f)
		v.c.With(labels).Add(f)
	}
}

/*
Collect is called by Prometheus at regular intervals to provide current data
*/
func (e *exporter) Collect(ch chan<- prometheus.Metric) {
	var elapsed time.Duration

	pollStatus := false

	mutex.Lock() // To protect metrics from concurrent collects.
	defer mutex.Unlock()

	log.Debugf("IBMMQ Collect started %o", ch)
	collectStartTime := time.Now()

	// If we're not connected, then continue to report a single metric about the qmgr status
	// This value is created by the mqmetric package even on z/OS which doesn't support
	// the DIS QMSTATUS command.
	if !isConnectedQMgr() {
		log.Infof("Reporting status as disconnected")
		if m, ok := qMgrStatusVecMap[mqmetric.ATTR_QMGR_STATUS]; ok {
			// There's no MQQMSTA_STOPPED value defined .All the regular qmgr status
			// constants start from 1. So we use "0" to indicate qmgr not available/stopped.
			// This must have the same set of tags as other qmgr-level metrics.
			desc := mqmetric.GetObjectDescription("", ibmmq.MQOT_Q_MGR)
			labels := prometheus.Labels{
				"qmgr":        strings.TrimSpace(config.cf.QMgrName),
				"description": desc,
				"platform":    platformString}

			// If we are able to get the qmgr's hostname, then use the last-known
			// one in this disconnected metric.
			// It will get replaced if the qmgr is doing a failover to a different machine.
			if supportsHostnameLabel() {
				if lastHostname == "" {
					lastHostname = mqmetric.DUMMY_STRING
				}
				labels["hostname"] = lastHostname
			}
			if showAndSupportsCustomLabel() {
				labels["custom"] = mqmetric.GetObjectCustom("", ibmmq.MQOT_Q_MGR)
			}
			addMetaLabels(labels)
			m.addMetric(labels, 0.0)
			m.Collect(ch)
		}
		return
	}

	// Clear out everything we know so far. In particular, replace
	// the map of values for each object so the collection starts
	// clean.
	for _, cl := range e.metrics.Classes {
		for _, ty := range cl.Types {
			for _, elem := range ty.Elements {
				ruaVecMap[makeKey(elem)].Reset()
				elem.Values = make(map[string]int64)
			}
		}
	}

	// Deal with all the publications that have arrived
	pubProcessTime := time.Now()
	err := mqmetric.ProcessPublications()
	pubProcessSecs := int64(time.Since(pubProcessTime).Seconds())
	if err != nil {
		log.Errorf("Error processing publications: %v", err)
	} else {
		log.Debugf("Collected and processed %d resource publications successfully in %d secs", mqmetric.GetProcessPublicationCount(), pubProcessSecs)
	}

	// Do we need to poll for object status on this iteration
	if err == nil {
		pollStatus = false
		thisPoll := time.Now()
		elapsed = thisPoll.Sub(lastPoll)
		if elapsed >= config.cf.PollIntervalDuration || isFirstCollection() {
			log.Debugf("Polling for object status")
			lastPoll = thisPoll
			pollStatus = true
		} else {
			log.Debugf("Skipping poll for object status")
		}

		// If there has been sufficient interval since the last explicit poll for
		// status, then do that collection too.
		// Collect any errors - normally we'd expect an error like the qmgr being
		// unavailable to have been picked up in the ProcessPublications call. But on
		// systems where you're not using the pub/sub metrics then errors here will be the
		// first ones found.
		// But here we may have unknown objects being referenced and while
		// it may be worth logging the error, that should not be cause to exit.
		pollError := err

		if pollStatus {
			for _, attr := range e.chlStatus.Attributes {
				channelStatusVecMap[attr.MetricName].Reset()
			}
			for _, attr := range e.qStatus.Attributes {
				qStatusVecMap[attr.MetricName].Reset()
			}
			for _, attr := range e.qMgrStatus.Attributes {
				qMgrStatusVecMap[attr.MetricName].Reset()
			}
			for _, attr := range e.topicStatus.Attributes {
				topicStatusVecMap[attr.MetricName].Reset()
			}
			for _, attr := range e.subStatus.Attributes {
				subStatusVecMap[attr.MetricName].Reset()
			}
			for _, attr := range e.clusterStatus.Attributes {
				clusterStatusVecMap[attr.MetricName].Reset()
			}

			if mqmetric.GetPlatform() == ibmmq.MQPL_ZOS {
				for _, attr := range e.usageBpStatus.Attributes {
					usageBpStatusVecMap[attr.MetricName].Reset()
				}
				for _, attr := range e.usagePsStatus.Attributes {
					usagePsStatusVecMap[attr.MetricName].Reset()
				}
			} else {
				for _, attr := range e.amqpStatus.Attributes {
					amqpStatusVecMap[attr.MetricName].Reset()
				}
				for _, attr := range e.mqttStatus.Attributes {
					mqttStatusVecMap[attr.MetricName].Reset()
				}
			}

			// Always collect the queue and qmgr status info regardless of the UseStatus flag, so that we keep
			// the known attribute values for tagging.
			if err == nil {
				err = mqmetric.CollectQueueStatus(config.cf.MonitoredQueues)
				if err != nil {
					log.Errorf("Error collecting queue status: %v", err)
					pollError = err
				} else {
					log.Debugf("Collected all queue status")
				}
			}

			if err == nil {
				err = mqmetric.CollectQueueManagerStatus()
				if err != nil {
					log.Errorf("Error collecting queue manager status: %v", err)
					pollError = err
				} else {
					log.Debugf("Collected all queue manager status")
				}
			}

			if config.cf.CC.UseStatus {
				if err == nil {
					err = mqmetric.CollectChannelStatus(config.cf.MonitoredChannels)
					if err != nil {
						log.Errorf("Error collecting channel status: %v", err)
						pollError = err
					} else {
						log.Debugf("Collected all channel status")
					}
				}

				if err == nil {
					err = mqmetric.CollectTopicStatus(config.cf.MonitoredTopics)
					if err != nil {
						log.Errorf("Error collecting topic status: %v", err)
						pollError = err
					} else {
						log.Debugf("Collected all topic status")
					}
				}

				if err == nil {
					err = mqmetric.CollectSubStatus(config.cf.MonitoredSubscriptions)
					if err != nil {
						log.Errorf("Error collecting subscription status: %v", err)
						pollError = err
					} else {
						log.Debugf("Collected all subscription status")
					}
				}

				/*				if err == nil {
									err = mqmetric.CollectQueueStatus(config.cf.MonitoredQueues)
									if err != nil {
										log.Errorf("Error collecting queue status: %v", err)
										pollError = err
									} else {
										log.Debugf("Collected all queue status")
									}
								}

								if err == nil {
									err = mqmetric.CollectQueueManagerStatus()
									if err != nil {
										log.Errorf("Error collecting queue manager status: %v", err)
										pollError = err
									} else {
										log.Debugf("Collected all queue manager status")
									}
								}
				*/
				if err == nil {
					err = mqmetric.CollectClusterStatus()
					if err != nil {
						log.Errorf("Error collecting cluster status: %v", err)
						pollError = err
					} else {
						log.Debugf("Collected all cluster status")
					}
				}

				if err == nil && mqmetric.GetPlatform() == ibmmq.MQPL_ZOS {
					err = mqmetric.CollectUsageStatus()
					if err != nil {
						log.Errorf("Error collecting bufferpool/pageset status: %v", err)
						pollError = err
					} else {
						log.Debugf("Collected all buffer pool/pageset status")
					}
				}

				if err == nil && mqmetric.GetPlatform() != ibmmq.MQPL_ZOS {
					if config.cf.MonitoredAMQPChannels != "" {
						err = mqmetric.CollectAMQPChannelStatus(config.cf.MonitoredAMQPChannels)
						if err != nil {
							log.Errorf("Error collecting AMQP channel status: %v", err)
							pollError = err
						} else {
							log.Debugf("Collected all AMQP channel status")
						}
					}

					if config.cf.MonitoredMQTTChannels != "" {
						err = mqmetric.CollectMQTTChannelStatus(config.cf.MonitoredMQTTChannels)
						if err != nil {
							log.Errorf("Error collecting MQTT channel status: %v", err)
							pollError = err
						} else {
							log.Debugf("Collected all MQTT channel status")
						}
					}
				}

			} else {
				log.Debugf("Not collecting object status")
			}
		}
		if err == nil {
			err = pollError
		}
	}

	// Possible enhancements: Be more discriminatory on errors that might
	// deserve a reconnection retry or which might be fatal
	if err != nil {
		log.Debugf("Exporter Error is %+v", err)

		mqrc := ibmmq.MQRC_NONE
		if mqe, ok := err.(mqmetric.MQMetricError); ok {
			mqrc = mqe.MQReturn.MQRC
		} else if mqe, ok := err.(*ibmmq.MQReturn); ok {
			mqrc = mqe.MQRC
		}

		// Almost any error should allow us to attempt to reconnect.
		// For example, ibmmq.MQRC_CONNECTION_BROKEN. But we might
		// want to put some additional error codes in here to explicitly
		// quit out of the collector.
		switch mqrc {
		case ibmmq.MQRC_NONE:
			setConnectedQMgr(true)
		case ibmmq.MQRC_UNEXPECTED_ERROR |
			ibmmq.MQRC_STANDBY_Q_MGR |
			ibmmq.MQRC_RECONNECT_FAILED:
			setCollectorEnd(true)
			setConnectedQMgr(false)
		default:
			setConnectedQMgr(false)
		}
		return
	}

	thisDiscovery := time.Now()
	elapsed = thisDiscovery.Sub(lastQueueDiscovery)
	if config.cf.RediscoverDuration > 0 {
		if elapsed >= config.cf.RediscoverDuration {
			log.Debugf("Doing queue rediscovery")
			_ = mqmetric.RediscoverAndSubscribe(discoverConfig)
			lastQueueDiscovery = thisDiscovery
			//if err == nil {
			_ = mqmetric.RediscoverAttributes(ibmmq.MQOT_CHANNEL, config.cf.MonitoredChannels)

			//}
			if mqmetric.GetPlatform() != ibmmq.MQPL_ZOS {
				_ = mqmetric.RediscoverAttributes(mqmetric.OT_CHANNEL_AMQP, config.cf.MonitoredAMQPChannels)
				_ = mqmetric.RediscoverAttributes(mqmetric.OT_CHANNEL_MQTT, config.cf.MonitoredMQTTChannels)
			}
		}
	}

	// Have now processed all of the publications, and all the MQ-owned
	// value fields and maps have been updated.
	//
	// Now need to set all of the real Gauges with the correct values
	if isFirstCollection() {
		// Always ignore the first loop through as there might
		// be accumulated stuff from a while ago, and lead to
		// a misleading range on graphs.
		setFirstCollection(false)
	} else {
		scrapeWarningPossible = true
		for _, cl := range e.metrics.Classes {
			for _, ty := range cl.Types {
				for _, elem := range ty.Elements {
					for key, value := range elem.Values {
						f := mqmetric.Normalise(elem, key, value)
						m := ruaVecMap[makeKey(elem)]
						if key == mqmetric.QMgrMapKey {
							desc := mqmetric.GetObjectDescription("", ibmmq.MQOT_Q_MGR)

							labels := prometheus.Labels{"qmgr": config.cf.QMgrName,
								"platform":    platformString,
								"description": desc}
							if supportsHostnameLabel() {
								// Stash the current hostname so it can be used in the "qmgr down" metric
								lastHostname = mqmetric.GetQueueManagerAttribute(config.cf.QMgrName, ibmmq.MQCACF_HOST_NAME)
								labels["hostname"] = lastHostname
							}
							if showAndSupportsCustomLabel() {
								labels["custom"] = mqmetric.GetObjectCustom("", ibmmq.MQOT_Q_MGR)
							}
							addMetaLabels(labels)
							m.addMetric(labels, f)
						} else if strings.HasPrefix(key, mqmetric.NativeHAKeyPrefix) {
							instanceName := strings.Replace(key, mqmetric.NativeHAKeyPrefix, "", -1)
							//log.Debugf("Adding NativeHA metric %s for %s", elem.MetricName, instanceName)
							labels := prometheus.Labels{"qmgr": config.cf.QMgrName,
								"nha":      instanceName,
								"platform": platformString}
							addMetaLabels(labels)
							m.addMetric(labels, f)
						} else { // It must be a queue
							usage := ""
							if usageAttr, ok := e.qStatus.Attributes[mqmetric.ATTR_Q_USAGE].Values[key]; ok {
								if usageAttr.ValueInt64 == int64(ibmmq.MQUS_TRANSMISSION) {
									usage = "XMITQ"
								} else {
									usage = "NORMAL"
								}
							} else {
								log.Debugf("Cannot find usage attr for %v", key)
							}

							// Don't submit metrics for queues where we've not done a full attribute discovery. Typically the first
							// collection period after a rediscover/resubscribe.
							if usage != "" {
								labels := prometheus.Labels{"qmgr": config.cf.QMgrName,
									"queue":       key,
									"usage":       usage,
									"description": mqmetric.GetObjectDescription(key, ibmmq.MQOT_Q),
									"cluster":     mqmetric.GetQueueAttribute(key, ibmmq.MQCA_CLUSTER_NAME),
									"platform":    platformString}
								if showAndSupportsCustomLabel() {
									labels["custom"] = mqmetric.GetObjectCustom(key, ibmmq.MQOT_Q)
								}
								addMetaLabels(labels)
								m.addMetric(labels, f)
							}

						}
					}
				}
			}
		}
	}

	// And tell Prometheus about the queue and qmgr data
	for _, cl := range e.metrics.Classes {
		for _, ty := range cl.Types {
			for _, elem := range ty.Elements {
				ruaVecMap[makeKey(elem)].Collect(ch)
				log.Debugf("Reporting metrics for %s", elem.MetricName)
			}
		}
	}
	// We will also send a pseudo-gauge that shows how many publications were processed
	if pubCountDesc == nil {
		fqName := prometheus.BuildFQName(config.namespace, "qmgr", "exporter_publications")
		pubCountDesc = prometheus.NewDesc(fqName,
			"How many resource publications processed",
			[]string{"qmgr", "platform"},
			nil)
	}
	// Tags must be in same order as created in the Description. But we don't need to have exactly the same tags
	// as all the other qmgr-level metrics
	ch <- prometheus.MustNewConstMetric(pubCountDesc, prometheus.GaugeValue, float64(mqmetric.GetProcessPublicationCount()), config.cf.QMgrName, platformString)

	// Next we extract the info for the object status metrics.
	if pollStatus {
		//Several of the attributes are used to build the tags that uniquely identify a channel instance
		for _, attr := range e.chlStatus.Attributes {
			for key, value := range attr.Values {
				if value.IsInt64 && !attr.Pseudo {
					m := channelStatusVecMap[attr.MetricName]
					f := mqmetric.ChannelNormalise(attr, value.ValueInt64)

					chlType := int(e.chlStatus.Attributes[mqmetric.ATTR_CHL_TYPE].Values[key].ValueInt64)
					chlTypeString := strings.Replace(ibmmq.MQItoString("CHT", chlType), "MQCHT_", "", -1)
					// Not every channel status report has the RQMNAME attribute (eg SVRCONNs)
					rqmname := "-"
					if rqmnameAttr, ok := e.chlStatus.Attributes[mqmetric.ATTR_CHL_RQMNAME].Values[key]; ok {
						rqmname = rqmnameAttr.ValueString
					}

					chlName := e.chlStatus.Attributes[mqmetric.ATTR_CHL_NAME].Values[key].ValueString
					connName := e.chlStatus.Attributes[mqmetric.ATTR_CHL_CONNNAME].Values[key].ValueString
					jobName := e.chlStatus.Attributes[mqmetric.ATTR_CHL_JOBNAME].Values[key].ValueString
					cipherSpec := mqmetric.DUMMY_STRING
					if cipherSpecAttr, ok := e.chlStatus.Attributes[mqmetric.ATTR_CHL_SSLCIPH].Values[key]; ok {
						cipherSpec = cipherSpecAttr.ValueString
					}
					log.Debugf("channel status - channelName: %s cipherSpec: \"%s\"", chlName, cipherSpec)
					// log.Debugf("channel status - key: %s channelName: %s metric: %s val: %v", key, chlName, attr.MetricName, f)
					labels := prometheus.Labels{
						"qmgr":                     strings.TrimSpace(config.cf.QMgrName),
						"channel":                  chlName,
						"platform":                 platformString,
						"description":              mqmetric.GetObjectDescription(chlName, ibmmq.MQOT_CHANNEL),
						mqmetric.ATTR_CHL_TYPE:     strings.TrimSpace(chlTypeString),
						mqmetric.ATTR_CHL_RQMNAME:  strings.TrimSpace(rqmname),
						mqmetric.ATTR_CHL_CONNNAME: strings.TrimSpace(connName),
						mqmetric.ATTR_CHL_JOBNAME:  strings.TrimSpace(jobName),
						mqmetric.ATTR_CHL_SSLCIPH:  strings.TrimSpace(cipherSpec)}

					addMetaLabels(labels)
					m.addMetric(labels, f)
				}
			}
		}

		for _, attr := range e.qStatus.Attributes {
			for key, value := range attr.Values {
				if value.IsInt64 && !attr.Pseudo {
					qName := e.qStatus.Attributes[mqmetric.ATTR_Q_NAME].Values[key].ValueString
					usage := ""
					if usageAttr, ok := e.qStatus.Attributes[mqmetric.ATTR_Q_USAGE].Values[key]; ok {
						if usageAttr.ValueInt64 == int64(ibmmq.MQUS_TRANSMISSION) {
							usage = "XMITQ"
						} else {
							usage = "NORMAL"
						}
					}

					m := qStatusVecMap[attr.MetricName]
					f := mqmetric.QueueNormalise(attr, value.ValueInt64)
					// log.Debugf("queue status - key: %s qName: %s metric: %s val: %v", key, qName, attr.MetricName, f)

					// Don't submit metrics for queues where we've not done a full attribute discovery. Typically the first
					// collection period after a rediscover/resubscribe.
					// Labels here must be the same as the queue labels in the published metrics collected and reported above
					if usage != "" {
						labels := prometheus.Labels{
							"qmgr":        strings.TrimSpace(config.cf.QMgrName),
							"platform":    platformString,
							"usage":       usage,
							"description": mqmetric.GetObjectDescription(qName, ibmmq.MQOT_Q),
							"cluster":     mqmetric.GetQueueAttribute(qName, ibmmq.MQCA_CLUSTER_NAME),
							"queue":       qName}
						if showAndSupportsCustomLabel() {
							labels["custom"] = mqmetric.GetObjectCustom(qName, ibmmq.MQOT_Q)
						}
						addMetaLabels(labels)
						m.addMetric(labels, f)
					}
				}
			}
		}

		for _, attr := range e.topicStatus.Attributes {
			for key, value := range attr.Values {
				if value.IsInt64 && !attr.Pseudo {
					topicString := e.topicStatus.Attributes[mqmetric.ATTR_TOPIC_STRING].Values[key].ValueString
					topicType := e.topicStatus.Attributes[mqmetric.ATTR_TOPIC_STATUS_TYPE].Values[key].ValueString
					m := topicStatusVecMap[attr.MetricName]
					f := mqmetric.TopicNormalise(attr, value.ValueInt64)
					labels := prometheus.Labels{
						"qmgr":     strings.TrimSpace(config.cf.QMgrName),
						"platform": platformString,
						"type":     topicType,
						"topic":    topicString}
					addMetaLabels(labels)
					m.addMetric(labels, f)
				}
			}
		}

		for _, attr := range e.subStatus.Attributes {
			for key, value := range attr.Values {
				if value.IsInt64 && !attr.Pseudo {
					subId := e.subStatus.Attributes[mqmetric.ATTR_SUB_ID].Values[key].ValueString
					subName := e.subStatus.Attributes[mqmetric.ATTR_SUB_NAME].Values[key].ValueString
					subType := int(e.subStatus.Attributes[mqmetric.ATTR_SUB_TYPE].Values[key].ValueInt64)
					subTypeString := strings.Replace(ibmmq.MQItoString("SUBTYPE", subType), "MQSUBTYPE_", "", -1)
					topicString := e.subStatus.Attributes[mqmetric.ATTR_SUB_TOPIC_STRING].Values[key].ValueString
					m := subStatusVecMap[attr.MetricName]
					f := mqmetric.SubNormalise(attr, value.ValueInt64)
					labels := prometheus.Labels{
						"qmgr":         strings.TrimSpace(config.cf.QMgrName),
						"platform":     platformString,
						"subid":        subId,
						"subscription": subName,
						"type":         subTypeString,
						"topic":        topicString}
					addMetaLabels(labels)
					m.addMetric(labels, f)
				}
			}
		}

		for _, attr := range e.qMgrStatus.Attributes {
			for _, value := range attr.Values {
				if value.IsInt64 && !attr.Pseudo {
					m := qMgrStatusVecMap[attr.MetricName]
					f := mqmetric.QueueManagerNormalise(attr, value.ValueInt64)
					desc := mqmetric.GetObjectDescription("", ibmmq.MQOT_Q_MGR)

					// Labels here must be the same as the qmgr labels in the published metrics collected and reported above
					labels := prometheus.Labels{
						"qmgr":        strings.TrimSpace(config.cf.QMgrName),
						"description": desc,
						"platform":    platformString}
					if supportsHostnameLabel() {
						labels["hostname"] = mqmetric.GetQueueManagerAttribute(config.cf.QMgrName, ibmmq.MQCACF_HOST_NAME)
					}
					if showAndSupportsCustomLabel() {
						labels["custom"] = mqmetric.GetObjectCustom("", ibmmq.MQOT_Q_MGR)
					}
					addMetaLabels(labels)
					m.addMetric(labels, f)
				}
			}
		}

		for _, attr := range e.clusterStatus.Attributes {
			for key, value := range attr.Values {
				clusterName := ""
				if cN, ok := e.clusterStatus.Attributes[mqmetric.ATTR_CLUSTER_NAME].Values[key]; ok {
					clusterName = cN.ValueString
				}

				qmType := int64(ibmmq.MQQMT_NORMAL)
				if qT, ok := e.clusterStatus.Attributes[mqmetric.ATTR_CLUSTER_QMTYPE].Values[key]; ok {
					qmType = qT.ValueInt64
				}
				qmTypeString := "PARTIAL"
				if qmType == int64(ibmmq.MQQMT_REPOSITORY) {
					qmTypeString = "FULL"
				}
				if value.IsInt64 && !attr.Pseudo {
					m := clusterStatusVecMap[attr.MetricName]
					f := mqmetric.ClusterNormalise(attr, value.ValueInt64)
					labels := prometheus.Labels{
						"qmgr":     strings.TrimSpace(config.cf.QMgrName),
						"cluster":  clusterName,
						"qmtype":   qmTypeString,
						"platform": platformString}
					addMetaLabels(labels)
					m.addMetric(labels, f)
				}
			}
		}

		if mqmetric.GetPlatform() == ibmmq.MQPL_ZOS {
			for _, attr := range e.usageBpStatus.Attributes {
				for key, value := range attr.Values {
					bpId := e.usageBpStatus.Attributes[mqmetric.ATTR_BP_ID].Values[key].ValueString
					bpLocation := e.usageBpStatus.Attributes[mqmetric.ATTR_BP_LOCATION].Values[key].ValueString
					bpClass := e.usageBpStatus.Attributes[mqmetric.ATTR_BP_CLASS].Values[key].ValueString
					if value.IsInt64 && !attr.Pseudo {
						m := usageBpStatusVecMap[attr.MetricName]
						f := mqmetric.UsageNormalise(attr, value.ValueInt64)
						labels := prometheus.Labels{
							"bufferpool": bpId,
							"location":   bpLocation,
							"pageclass":  bpClass,
							"qmgr":       strings.TrimSpace(config.cf.QMgrName),
							"platform":   platformString}
						addMetaLabels(labels)
						m.addMetric(labels, f)
					}
				}
			}

			for _, attr := range e.usagePsStatus.Attributes {
				for key, value := range attr.Values {
					psId := e.usagePsStatus.Attributes[mqmetric.ATTR_PS_ID].Values[key].ValueString
					bpId := e.usagePsStatus.Attributes[mqmetric.ATTR_PS_BPID].Values[key].ValueString
					if value.IsInt64 && !attr.Pseudo {
						m := usagePsStatusVecMap[attr.MetricName]
						f := mqmetric.UsageNormalise(attr, value.ValueInt64)
						labels := prometheus.Labels{
							"pageset":    psId,
							"bufferpool": bpId,
							"qmgr":       strings.TrimSpace(config.cf.QMgrName),
							"platform":   platformString}
						addMetaLabels(labels)
						m.addMetric(labels, f)
					}
				}
			}
		} else {
			for _, attr := range e.amqpStatus.Attributes {
				for key, value := range attr.Values {
					chlName := e.amqpStatus.Attributes[mqmetric.ATTR_CHL_NAME].Values[key].ValueString
					clientId := e.amqpStatus.Attributes[mqmetric.ATTR_CHL_AMQP_CLIENT_ID].Values[key].ValueString
					connName := e.amqpStatus.Attributes[mqmetric.ATTR_CHL_CONNNAME].Values[key].ValueString
					if value.IsInt64 && !attr.Pseudo {
						m := amqpStatusVecMap[attr.MetricName]
						f := mqmetric.ChannelNormalise(attr, value.ValueInt64)
						labels := prometheus.Labels{
							"qmgr":                           strings.TrimSpace(config.cf.QMgrName),
							"channel":                        chlName,
							"description":                    mqmetric.GetObjectDescription(chlName, mqmetric.OT_CHANNEL_AMQP),
							"platform":                       platformString,
							mqmetric.ATTR_CHL_AMQP_CLIENT_ID: clientId,
							mqmetric.ATTR_CHL_CONNNAME:       strings.TrimSpace(connName)}
						addMetaLabels(labels)
						m.addMetric(labels, f)

					}
				}
			}

			for _, attr := range e.mqttStatus.Attributes {
				for key, value := range attr.Values {
					chlName := e.mqttStatus.Attributes[mqmetric.ATTR_CHL_NAME].Values[key].ValueString
					clientId := e.mqttStatus.Attributes[mqmetric.ATTR_CHL_MQTT_CLIENT_ID].Values[key].ValueString
					connName := e.mqttStatus.Attributes[mqmetric.ATTR_CHL_CONNNAME].Values[key].ValueString
					if value.IsInt64 && !attr.Pseudo {
						m := mqttStatusVecMap[attr.MetricName]
						f := mqmetric.ChannelNormalise(attr, value.ValueInt64)
						labels := prometheus.Labels{
							"qmgr":                           strings.TrimSpace(config.cf.QMgrName),
							"channel":                        chlName,
							"description":                    mqmetric.GetObjectDescription(chlName, mqmetric.OT_CHANNEL_MQTT),
							"platform":                       platformString,
							mqmetric.ATTR_CHL_MQTT_CLIENT_ID: clientId,
							mqmetric.ATTR_CHL_CONNNAME:       strings.TrimSpace(connName)}
						addMetaLabels(labels)
						m.addMetric(labels, f)

					}
				}
			}
		}
	}

	// Then put the responses from DIS xxSTATUS info back to Prometheus
	// We do this even if we have not polled for new status, so that Grafana's "instant"
	// view will still show up the most recently known values
	for _, attr := range e.chlStatus.Attributes {
		if !attr.Pseudo {
			m := channelStatusVecMap[attr.MetricName]
			log.Debugf("Reporting chl   metrics for %s", attr.MetricName)
			m.CollectWrap(ch)
		}
	}
	for _, attr := range e.qStatus.Attributes {
		if !attr.Pseudo {
			m := qStatusVecMap[attr.MetricName]
			log.Debugf("Reporting queue metrics for %s", attr.MetricName)
			m.CollectWrap(ch)
		}
	}
	for _, attr := range e.topicStatus.Attributes {
		if !attr.Pseudo {
			m := topicStatusVecMap[attr.MetricName]
			//log.Debugf("Reporting topic metrics for %s", attr.MetricName)
			m.CollectWrap(ch)
		}
	}
	for _, attr := range e.subStatus.Attributes {
		if !attr.Pseudo {
			m := subStatusVecMap[attr.MetricName]
			log.Debugf("Reporting subs  metrics for %s", attr.MetricName)
			m.CollectWrap(ch)
		}
	}
	for _, attr := range e.qMgrStatus.Attributes {
		if !attr.Pseudo {
			m := qMgrStatusVecMap[attr.MetricName]
			log.Debugf("Reporting qmgr  metrics for %s", attr.MetricName)
			m.CollectWrap(ch)
		}
	}

	for _, attr := range e.clusterStatus.Attributes {
		if !attr.Pseudo {
			m := clusterStatusVecMap[attr.MetricName]
			log.Debugf("Reporting cluster  metrics for %s", attr.MetricName)
			m.CollectWrap(ch)
		}
	}

	if mqmetric.GetPlatform() == ibmmq.MQPL_ZOS {
		for _, attr := range e.usageBpStatus.Attributes {
			if !attr.Pseudo {
				m := usageBpStatusVecMap[attr.MetricName]
				log.Debugf("Reporting BPool metrics for %s", attr.MetricName)
				m.CollectWrap(ch)
			}
		}
		for _, attr := range e.usagePsStatus.Attributes {
			if !attr.Pseudo {
				m := usagePsStatusVecMap[attr.MetricName]
				log.Debugf("Reporting Pageset metrics for %s", attr.MetricName)
				m.CollectWrap(ch)
			}
		}
	} else {
		for _, attr := range e.amqpStatus.Attributes {
			if !attr.Pseudo {
				m := amqpStatusVecMap[attr.MetricName]
				log.Debugf("Reporting AMQP metrics for %s", attr.MetricName)
				m.CollectWrap(ch)
			}
		}

		for _, attr := range e.mqttStatus.Attributes {
			if !attr.Pseudo {
				m := mqttStatusVecMap[attr.MetricName]
				log.Debugf("Reporting MQTT metrics for %s", attr.MetricName)
				m.CollectWrap(ch)
			}
		}
	}

	collectStopTime := time.Now()
	elapsedSecs := int64(collectStopTime.Sub(collectStartTime).Seconds())
	log.Debugf("Collection time = %d secs", elapsedSecs)

	// Issue a warning if it looks like we've exceeded the default scrape_timeout. Don't do it on the first full iteration as that
	// appears to sometimes be quite a bit slower anyway.
	if elapsedSecs > defaultScrapeTimeout && !scrapeWarningIssued && scrapeWarningPossible {
		log.Warnf("Collection time (%d secs) has exceeded Prometheus default scrape_timeout value of %d seconds. Ensure you have set a larger value for this job.", elapsedSecs, defaultScrapeTimeout)
		scrapeWarningIssued = true
	}
	// We will also send a pseudo-gauge that shows how long it took to process the collection request.
	// Note that this is going to be less than the value reported by Prometheus itself because the flow back
	// to the database may still be going on in another background thread
	if collectionTimeDesc == nil {
		fqName := prometheus.BuildFQName(config.namespace, "qmgr", "exporter_collection_time")
		collectionTimeDesc = prometheus.NewDesc(fqName,
			"How long the last collection took",
			[]string{"qmgr", "platform"},
			nil)
	}
	// Tags must be in same order as created in the Description. But we don't need to have exactly the same tags
	// as all the other qmgr-level metrics
	ch <- prometheus.MustNewConstMetric(collectionTimeDesc, prometheus.GaugeValue, float64(elapsedSecs), config.cf.QMgrName, platformString)

	// Break out after a small number of iterations when we are testing
	// log.Debugf("Max loops: %d Cur loops: %d", unittestMaxLoops, unittestLoops)
	if unittestMaxLoops != 0 {
		unittestLoops++
		if unittestLoops > unittestMaxLoops {
			log.Infof("Maximum unittest iterations of %d reached", unittestMaxLoops)
			os.Exit(0)
		}
	}
}

func allocateAllGauges() {
	log.Debugf("About to allocate gauges")
	allocateGauges()
	log.Debugf("PubSub Gauges allocated")
	allocateChannelStatusGauges()
	log.Debugf("ChannelGauges allocated")
	allocateQStatusGauges()
	log.Debugf("Queue  Gauges allocated")
	allocateTopicStatusGauges()
	log.Debugf("Topic  Gauges allocated")
	allocateSubStatusGauges()
	log.Debugf("Subscription Gauges allocated")
	allocateQMgrStatusGauges()
	log.Debugf("QMgr   Gauges allocated")
	allocateClusterStatusGauges()
	log.Debugf("cluster Gauges allocated")
	if mqmetric.GetPlatform() == ibmmq.MQPL_ZOS {
		allocateUsageStatusGauges()
		log.Debugf("BP/PS  Gauges allocated")
	} else {
		allocateAMQPStatusGauges()
		log.Debugf("AMQP  Gauges allocated")
		allocateMQTTStatusGauges()
		log.Debugf("MQTT  Gauges allocated")
	}
}

/*
allocateGauges creates a Prometheus gauge for each
resource that we know about. These are stored in a local map keyed
from the resource names.
*/
func allocateGauges() {
	for _, cl := range mqmetric.GetPublishedMetrics("").Classes {
		for _, ty := range cl.Types {
			for _, elem := range ty.Elements {
				m := newMqVec(elem)
				key := makeKey(elem)
				ruaVecMap[key] = m
			}
		}
	}
}

func allocateChannelStatusGauges() {
	// These attributes do not (currently) have an NLS translated description
	mqmetric.ChannelInitAttributes()
	for _, attr := range mqmetric.GetObjectStatus("", mqmetric.OT_CHANNEL).Attributes {
		m := newMqVecObj(attr, "channel")
		channelStatusVecMap[attr.MetricName] = m
	}
}

func allocateAMQPStatusGauges() {
	mqmetric.ChannelAMQPInitAttributes()
	for _, attr := range mqmetric.GetObjectStatus("", mqmetric.OT_CHANNEL_AMQP).Attributes {
		m := newMqVecObj(attr, "amqp")
		amqpStatusVecMap[attr.MetricName] = m
	}
}
func allocateMQTTStatusGauges() {
	mqmetric.ChannelMQTTInitAttributes()
	for _, attr := range mqmetric.GetObjectStatus("", mqmetric.OT_CHANNEL_MQTT).Attributes {
		m := newMqVecObj(attr, "mqtt")
		mqttStatusVecMap[attr.MetricName] = m
	}
}
func allocateQStatusGauges() {
	mqmetric.QueueInitAttributes()
	for _, attr := range mqmetric.GetObjectStatus("", mqmetric.OT_Q).Attributes {
		m := newMqVecObj(attr, "queue")
		qStatusVecMap[attr.MetricName] = m
	}
}

func allocateTopicStatusGauges() {
	mqmetric.TopicInitAttributes()
	for _, attr := range mqmetric.GetObjectStatus("", mqmetric.OT_TOPIC).Attributes {
		m := newMqVecObj(attr, "topic")
		topicStatusVecMap[attr.MetricName] = m
	}
}

func allocateSubStatusGauges() {
	mqmetric.SubInitAttributes()
	for _, attr := range mqmetric.GetObjectStatus("", mqmetric.OT_SUB).Attributes {
		m := newMqVecObj(attr, "subscription")
		subStatusVecMap[attr.MetricName] = m
	}
}

func allocateQMgrStatusGauges() {
	mqmetric.QueueManagerInitAttributes()
	for _, attr := range mqmetric.GetObjectStatus("", mqmetric.OT_Q_MGR).Attributes {
		m := newMqVecObj(attr, "qmgr")
		qMgrStatusVecMap[attr.MetricName] = m
	}
}

func allocateClusterStatusGauges() {
	mqmetric.ClusterInitAttributes()
	for _, attr := range mqmetric.GetObjectStatus("", mqmetric.OT_CLUSTER).Attributes {
		m := newMqVecObj(attr, "cluster")
		clusterStatusVecMap[attr.MetricName] = m
	}
}

func allocateUsageStatusGauges() {
	mqmetric.UsageInitAttributes()
	for _, attr := range mqmetric.GetObjectStatus("", mqmetric.OT_BP).Attributes {
		m := newMqVecObj(attr, "bufferpool")
		usageBpStatusVecMap[attr.MetricName] = m
	}
	for _, attr := range mqmetric.GetObjectStatus("", mqmetric.OT_PS).Attributes {
		m := newMqVecObj(attr, "pageset")
		usagePsStatusVecMap[attr.MetricName] = m
	}
}

/*
makeKey uses the 3 parts of a resource's name to build a unique string.
The "/" character cannot be part of a name, so is a convenient way
to build a unique key. If we ever have metrics for other object
types such as topics, then the object type would be used too.
This key is not used outside of this module, so the format can change.
*/
func makeKey(elem *mqmetric.MonElement) string {
	key := elem.Parent.Parent.Name + "/" +
		elem.Parent.Name + "/" +
		elem.MetricName
	return key
}

/*
newMqVec returns the structure which will contain the
values and suitable labels. These tags have to all be used
when the metrics are collected by Prometheus.
*/
func newMqVec(elem *mqmetric.MonElement) *MQVec {
	queueLabelNames := []string{"queue", "qmgr", "platform", "usage", "description", "cluster"}
	if showAndSupportsCustomLabel() {
		queueLabelNames = append(queueLabelNames, "custom")
	}
	nhaLabelNames := []string{"qmgr", "platform", "nha"}
	// If the qmgr tags change, then check the special metric indicating qmgr unavailable as that's
	// not part of the regular collection blocks.
	// "Hostname" was added to DIS QMSTATUS on Distributed platforms at version 9.3.2
	qmgrLabelNames := []string{"qmgr", "platform", "description"}
	if supportsHostnameLabel() {
		qmgrLabelNames = append(qmgrLabelNames, "hostname")
	}
	if showAndSupportsCustomLabel() {
		qmgrLabelNames = append(qmgrLabelNames, "custom")
	}
	labels := qmgrLabelNames
	prefix := "qmgr_"

	ot := elem.Parent.ObjectTopic
	if strings.Contains(ot, "%s") {
		if strings.Contains(ot, "/NHAREPLICA/") {
			labels = nhaLabelNames
			prefix = "nha_"
		} else {
			labels = queueLabelNames
			prefix = "queue_"
		}
	}

	if len(labels) > 0 {
		for i := 0; i < len(config.cf.MetadataTagsArray); i++ {
			labels = append(labels, config.cf.MetadataTagsArray[i])
		}
	}

	// After the change that makes the prefix "queue" to indicate the object type (instead of
	// "object", then there are some metrics that look a bit silly such as
	// "queue_queue_purged". So we remove the duplicate.
	if prefix == "queue_" && strings.HasPrefix(elem.MetricName, "queue_") {
		prefix = ""
	}

	description := elem.DescriptionNLS
	if description == "" {
		description = elem.Description
	}

	vecType := "Counter"
	mqVec := new(MQVec)
	delta := false
	if elem.Datatype == ibmmq.MQIAMO_MONITOR_DELTA {
		delta = true
	}

	name := elem.MetricName

	// Create either a Counter or a Gauge. For historic reasons, everything used
	// to be a Gauge. Because the change might affect any dashboards you have created.
	// you have to explicitly opt in to splitting the types.
	if config.overrideCTypeBool && delta {
		counterVec := prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: config.namespace,
				Name:      prefix + name,
				Help:      description,
			},
			labels,
		)
		mqVec.c = counterVec
		mqVec.n = prefix + name

	} else {
		vecType = "Gauge  "
		gaugeVec := prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Namespace: config.namespace,
				Name:      prefix + name,
				Help:      description,
			},
			labels,
		)
		mqVec.g = gaugeVec
		mqVec.n = prefix + name

	}
	log.Debugf("Created %s for '%s%s' from '%s'", vecType, prefix, name, elem.Description)

	return mqVec
}

/*
 * Create counters/gauges for other object types. The status for these is obtained via
 * a polling mechanism rather than pub/sub.
 */
func newMqVecObj(attr *mqmetric.StatusAttribute, objectType string) *MQVec {
	var labels []string

	prefix := objectType + "_"
	description := attr.Description
	name := attr.MetricName

	// There can be several channels active of the same name. They can be independently
	// identified by the MCAJobName attribute along with connName. So those are set as labels
	// on the gauge. The remote qmgr is also useful information to know.
	channelLabels := []string{"qmgr", "platform", objectType, "description",
		mqmetric.ATTR_CHL_TYPE,
		mqmetric.ATTR_CHL_RQMNAME,
		mqmetric.ATTR_CHL_CONNNAME,
		mqmetric.ATTR_CHL_JOBNAME,
		mqmetric.ATTR_CHL_SSLCIPH}

	// These labels have to be the same set as those used by the published
	// resources.
	qmgrLabels := []string{"qmgr", "platform", "description"}
	if supportsHostnameLabel() {
		qmgrLabels = append(qmgrLabels, "hostname")
	}
	if showAndSupportsCustomLabel() {
		qmgrLabels = append(qmgrLabels, "custom")
	}
	// With topic status, need to know if type is "pub" or "sub"
	topicLabels := []string{"qmgr", "platform", objectType, "type"}
	subLabels := []string{"qmgr", "platform", objectType, "subid", "topic", "type"}
	bpLabels := []string{"qmgr", "platform", objectType, "location", "pageclass"}
	psLabels := []string{"qmgr", "platform", objectType, "bufferpool"}
	clusterLabels := []string{"qmgr", "platform", "cluster", "qmtype"}
	amqpLabels := []string{"qmgr", "platform", "description", "channel",
		mqmetric.ATTR_CHL_AMQP_CLIENT_ID,
		mqmetric.ATTR_CHL_CONNNAME}
	mqttLabels := []string{"qmgr", "platform", "description", "channel",
		mqmetric.ATTR_CHL_MQTT_CLIENT_ID,
		mqmetric.ATTR_CHL_CONNNAME}
	// Adding the polling queue status options means we can use this block for
	// additional attributes. They should have the same labels as the stats generated
	// through resource publications.
	queueLabels := []string{"qmgr", "platform", objectType, "usage", "description", "cluster"}
	if showAndSupportsCustomLabel() {
		queueLabels = append(queueLabels, "custom")
	}

	switch objectType {
	case "channel":
		labels = channelLabels
	case "topic":
		labels = topicLabels
	case "subscription":
		labels = subLabels
	case "queue":
		labels = queueLabels
	case "qmgr":
		labels = qmgrLabels
	case "bufferpool":
		labels = bpLabels
	case "pageset":
		labels = psLabels
	case "cluster":
		labels = clusterLabels
	case "amqp":
		labels = amqpLabels
	case "mqtt":
		labels = mqttLabels
	default:
		log.Errorf("Tried to create metrics for unknown object type %s", objectType)
	}

	if len(labels) > 0 {
		for i := 0; i < len(config.cf.MetadataTagsArray); i++ {
			labels = append(labels, config.cf.MetadataTagsArray[i])
		}
	}

	m := new(MQVec)

	// Create either a Counter or a Gauge. For historic reasons, everything used
	// to be a Gauge. Because the change might affect any dashboards you have created.
	// you have to explicitly opt in to splitting the types.
	vecType := "Counter"
	if config.overrideCTypeBool && attr.Delta {
		counterVec := prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: config.namespace,
				Name:      prefix + name,
				Help:      description,
			},
			labels,
		)
		m.c = counterVec
		m.n = prefix + name

	} else {
		vecType = "Gauge  "
		gaugeVec := prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Namespace: config.namespace,
				Name:      prefix + name,
				Help:      description,
			},
			labels,
		)
		m.g = gaugeVec
		m.n = prefix + name
	}
	log.Debugf("Created %s for '%s%s' from '%s'", vecType, prefix, name, description)

	return m
}

func supportsHostnameLabel() bool {
	rc := false
	// First time through, work out whether we're connected to a qmgr that has the
	// hostname status attribute. Stash that so it doesn't get reset if we're in a reconnect
	// sequence
	if supportsHostnameLabelVal == nil {
		if mqmetric.GetPlatform() != ibmmq.MQPL_ZOS && mqmetric.GetCommandLevel() >= ibmmq.MQCMDL_LEVEL_932 {
			rc = true
		}

		supportsHostnameLabelVal = new(bool)
		*supportsHostnameLabelVal = rc
	} else {
		rc = *supportsHostnameLabelVal
	}
	//log.Debugf("supportsHostnameLabel: %v", rc)
	return rc
}

func showAndSupportsCustomLabel() bool {
	return config.cf.CC.ShowCustomAttribute
}

func addMetaLabels(labels prometheus.Labels) {
	if len(config.cf.MetadataTagsArray) > 0 {
		for i := 0; i < len(config.cf.MetadataTagsArray); i++ {
			labels[config.cf.MetadataTagsArray[i]] = config.cf.MetadataValuesArray[i]
		}
	}
}
