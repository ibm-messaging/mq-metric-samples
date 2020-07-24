package main

/*
  Copyright (c) IBM Corporation 2016, 2019

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
	"strings"
	"sync"
	"time"

	"github.com/ibm-messaging/mq-golang/v5/ibmmq"
	"github.com/ibm-messaging/mq-golang/v5/mqmetric"
	"github.com/prometheus/client_golang/prometheus"
	log "github.com/sirupsen/logrus"
)

type exporter struct {
	mutex         sync.RWMutex
	metrics       mqmetric.AllMetrics
	chlStatus     mqmetric.StatusSet
	qStatus       mqmetric.StatusSet
	topicStatus   mqmetric.StatusSet
	subStatus     mqmetric.StatusSet
	qMgrStatus    mqmetric.StatusSet
	usageBpStatus mqmetric.StatusSet
	usagePsStatus mqmetric.StatusSet
}

func newExporter() *exporter {
	return &exporter{
		metrics:       mqmetric.Metrics,
		chlStatus:     mqmetric.ChannelStatus,
		qStatus:       mqmetric.QueueStatus,
		topicStatus:   mqmetric.TopicStatus,
		subStatus:     mqmetric.SubStatus,
		qMgrStatus:    mqmetric.QueueManagerStatus,
		usageBpStatus: mqmetric.UsageBpStatus,
		usagePsStatus: mqmetric.UsagePsStatus,
	}
}

const (
	defaultScrapeTimeout = 10 // Prometheus default scrape_timeout is 10s
)

var (
	first                 = true
	gaugeMap              = make(map[string]*prometheus.GaugeVec)
	channelStatusGaugeMap = make(map[string]*prometheus.GaugeVec)
	qStatusGaugeMap       = make(map[string]*prometheus.GaugeVec)
	topicStatusGaugeMap   = make(map[string]*prometheus.GaugeVec)
	subStatusGaugeMap     = make(map[string]*prometheus.GaugeVec)
	qMgrStatusGaugeMap    = make(map[string]*prometheus.GaugeVec)
	usageBpStatusGaugeMap = make(map[string]*prometheus.GaugeVec)
	usagePsStatusGaugeMap = make(map[string]*prometheus.GaugeVec)
	lastPoll              = time.Now()
	lastQueueDiscovery    time.Time
	platformString        string
	counter               = 0
	warnedScrapeTimeout   = false
	pubCountDesc          *prometheus.Desc
)

/*
Describe is called by Prometheus on startup of this monitor. It needs to tell
the caller about all of the available metrics.
*/
func (e *exporter) Describe(ch chan<- *prometheus.Desc) {

	log.Infof("IBMMQ Describe started")

	platformString = strings.Replace(ibmmq.MQItoString("PL", int(mqmetric.GetPlatform())), "MQPL_", "", -1)
	log.Infof("Platform is %s", platformString)
	for _, cl := range e.metrics.Classes {
		for _, ty := range cl.Types {
			for _, elem := range ty.Elements {
				gaugeMap[makeKey(elem)].Describe(ch)
			}
		}
	}

	for _, attr := range e.chlStatus.Attributes {
		channelStatusGaugeMap[attr.MetricName].Describe(ch)
	}
	for _, attr := range e.qStatus.Attributes {
		qStatusGaugeMap[attr.MetricName].Describe(ch)
	}
	for _, attr := range e.topicStatus.Attributes {
		topicStatusGaugeMap[attr.MetricName].Describe(ch)
	}
	for _, attr := range e.subStatus.Attributes {
		subStatusGaugeMap[attr.MetricName].Describe(ch)
	}

	// DISPLAY QMSTATUS is not supported on z/OS
	// but we do extract a couple of MQINQable attributes
	for _, attr := range e.qMgrStatus.Attributes {
		qMgrStatusGaugeMap[attr.MetricName].Describe(ch)
	}

	// The BufferPool and PageSet stuff is only for z/OS
	if mqmetric.GetPlatform() == ibmmq.MQPL_ZOS {
		for _, attr := range e.usageBpStatus.Attributes {
			usageBpStatusGaugeMap[attr.MetricName].Describe(ch)
		}
		for _, attr := range e.usagePsStatus.Attributes {
			usagePsStatusGaugeMap[attr.MetricName].Describe(ch)
		}
	}
}

/*
Collect is called by Prometheus at regular intervals to provide current
data
*/
func (e *exporter) Collect(ch chan<- prometheus.Metric) {
	e.mutex.Lock() // To protect metrics from concurrent collects.
	defer e.mutex.Unlock()

	log.Infof("IBMMQ Collect started %o", ch)
	collectStartTime := time.Now()

	// Clear out everything we know so far. In particular, replace
	// the map of values for each object so the collection starts
	// clean.
	for _, cl := range e.metrics.Classes {
		for _, ty := range cl.Types {
			for _, elem := range ty.Elements {
				gaugeMap[makeKey(elem)].Reset()
				elem.Values = make(map[string]int64)
			}
		}
	}

	// Deal with all the publications that have arrived
	err := mqmetric.ProcessPublications()
	if err != nil {
		log.Fatalf("Error processing publications: %v", err)
	} else {
		log.Debugf("Collected and processed resource publications successfully")
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
	// If there has been sufficient interval since the last explicit poll for
	// status, then do that collection too. Don't treat errors in this block
	// as fatal - the previous section will have caught things like the qmgr
	// going down. But here we may have unknown objects being referenced and while
	// it may be worth logging the error, that should not be cause to exit.
	if pollStatus {
		for _, attr := range e.chlStatus.Attributes {
			channelStatusGaugeMap[attr.MetricName].Reset()
		}
		for _, attr := range e.qStatus.Attributes {
			qStatusGaugeMap[attr.MetricName].Reset()
		}
		for _, attr := range e.qMgrStatus.Attributes {
			qMgrStatusGaugeMap[attr.MetricName].Reset()
		}
		for _, attr := range e.topicStatus.Attributes {
			topicStatusGaugeMap[attr.MetricName].Reset()
		}
		for _, attr := range e.subStatus.Attributes {
			subStatusGaugeMap[attr.MetricName].Reset()
		}

		if config.cf.CC.UseStatus {
			err := mqmetric.CollectChannelStatus(config.cf.MonitoredChannels)
			if err != nil {
				log.Errorf("Error collecting channel status: %v", err)
			} else {
				log.Debugf("Collected all channel status")
			}

			err = mqmetric.CollectTopicStatus(config.cf.MonitoredTopics)
			if err != nil {
				log.Errorf("Error collecting topic status: %v", err)
			} else {
				log.Debugf("Collected all topic status")
			}

			err = mqmetric.CollectSubStatus(config.cf.MonitoredSubscriptions)
			if err != nil {
				log.Errorf("Error collecting subscription status: %v", err)
			} else {
				log.Debugf("Collected all subscription status")
			}

			err = mqmetric.CollectQueueStatus(config.cf.MonitoredQueues)
			if err != nil {
				log.Errorf("Error collecting queue status: %v", err)
			} else {
				log.Debugf("Collected all queue status")
			}
		}

		err = mqmetric.CollectQueueManagerStatus()
		if err != nil {
			log.Errorf("Error collecting queue manager status: %v", err)
		} else {
			log.Debugf("Collected all queue manager status")
		}

		if mqmetric.GetPlatform() == ibmmq.MQPL_ZOS {
			err = mqmetric.CollectUsageStatus()
			if err != nil {
				log.Errorf("Error collecting bufferpool/pageset status: %v", err)
			} else {
				log.Debugf("Collected all buffer pool/pageset status")
			}
		}
	}

	thisDiscovery := time.Now()
	elapsed = thisDiscovery.Sub(lastQueueDiscovery)
	if config.cf.RediscoverDuration > 0 {
		if elapsed >= config.cf.RediscoverDuration {
			log.Debugf("Doing queue rediscovery")
			err = mqmetric.RediscoverAndSubscribe(discoverConfig)
			lastQueueDiscovery = thisDiscovery
			//if err == nil {
			err = mqmetric.RediscoverAttributes(ibmmq.MQOT_CHANNEL, config.cf.MonitoredChannels)
			//}
		}
	}

	// Have now processed all of the publications, and all the MQ-owned
	// value fields and maps have been updated.
	//
	// Now need to set all of the real Gauges with the correct values
	if first {
		// Always ignore the first loop through as there might
		// be accumulated stuff from a while ago, and lead to
		// a misleading range on graphs.
		first = false
	} else {

		for _, cl := range e.metrics.Classes {
			for _, ty := range cl.Types {
				for _, elem := range ty.Elements {
					for key, value := range elem.Values {
						f := mqmetric.Normalise(elem, key, value)
						g := gaugeMap[makeKey(elem)]
						if key == mqmetric.QMgrMapKey {
							g.With(prometheus.Labels{"qmgr": config.cf.QMgrName,
								"platform": platformString}).Set(f)
						} else {
							usage := ""
							if usageAttr, ok := e.qStatus.Attributes[mqmetric.ATTR_Q_USAGE].Values[key]; ok {
								if usageAttr.ValueInt64 == 1 {
									usage = "XMITQ"
								} else {
									usage = "NORMAL"
								}
							}
							g.With(prometheus.Labels{"qmgr": config.cf.QMgrName,
								"queue":       key,
								"usage":       usage,
								"description": mqmetric.GetObjectDescription(key, ibmmq.MQOT_Q),
								"platform":    platformString}).Set(f)
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
				gaugeMap[makeKey(elem)].Collect(ch)
				log.Debugf("Reporting gauge for %s", elem.MetricName)
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
	// Tags must be in same order as created in the Description
	ch <- prometheus.MustNewConstMetric(pubCountDesc, prometheus.GaugeValue, float64(mqmetric.GetProcessPublicationCount()), config.cf.QMgrName, platformString)

	// Next we extract the info for channel status. Several of the attributes
	// are used to build the tags that uniquely identify a channel instance
	if pollStatus {
		for _, attr := range e.chlStatus.Attributes {
			for key, value := range attr.Values {
				if value.IsInt64 && !attr.Pseudo {
					g := channelStatusGaugeMap[attr.MetricName]

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

					log.Debugf("channel status - key: %s channelName: %s metric: %s val: %v", key, chlName, attr.MetricName, f)

					g.With(prometheus.Labels{
						"qmgr":                     strings.TrimSpace(config.cf.QMgrName),
						"channel":                  chlName,
						"platform":                 platformString,
						"description":              mqmetric.GetObjectDescription(chlName, ibmmq.MQOT_CHANNEL),
						mqmetric.ATTR_CHL_TYPE:     strings.TrimSpace(chlTypeString),
						mqmetric.ATTR_CHL_RQMNAME:  strings.TrimSpace(rqmname),
						mqmetric.ATTR_CHL_CONNNAME: strings.TrimSpace(connName),
						mqmetric.ATTR_CHL_JOBNAME:  strings.TrimSpace(jobName)}).Set(f)
				}
			}
		}

		for _, attr := range e.qStatus.Attributes {
			for key, value := range attr.Values {
				if value.IsInt64 && !attr.Pseudo {
					qName := e.qStatus.Attributes[mqmetric.ATTR_Q_NAME].Values[key].ValueString
					usage := "NORMAL"
					if usageAttr, ok := e.qStatus.Attributes[mqmetric.ATTR_Q_USAGE].Values[key]; ok {
						if usageAttr.ValueInt64 == 1 {
							usage = "XMITQ"
						} else {
							usage = "NORMAL"
						}
					}

					g := qStatusGaugeMap[attr.MetricName]
					f := mqmetric.QueueNormalise(attr, value.ValueInt64)
					log.Debugf("queue status - key: %s qName: %s metric: %s val: %v", key, qName, attr.MetricName, f)

					g.With(prometheus.Labels{
						"qmgr":        strings.TrimSpace(config.cf.QMgrName),
						"platform":    platformString,
						"usage":       usage,
						"description": mqmetric.GetObjectDescription(qName, ibmmq.MQOT_Q),
						"queue":       qName}).Set(f)
				}
			}
		}

		for _, attr := range e.topicStatus.Attributes {
			for key, value := range attr.Values {
				if value.IsInt64 && !attr.Pseudo {
					topicString := e.topicStatus.Attributes[mqmetric.ATTR_TOPIC_STRING].Values[key].ValueString
					topicType := e.topicStatus.Attributes[mqmetric.ATTR_TOPIC_STATUS_TYPE].Values[key].ValueString
					g := topicStatusGaugeMap[attr.MetricName]
					f := mqmetric.TopicNormalise(attr, value.ValueInt64)

					g.With(prometheus.Labels{
						"qmgr":     strings.TrimSpace(config.cf.QMgrName),
						"platform": platformString,
						"type":     topicType,
						"topic":    topicString}).Set(f)
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
					g := subStatusGaugeMap[attr.MetricName]
					f := mqmetric.SubNormalise(attr, value.ValueInt64)

					g.With(prometheus.Labels{
						"qmgr":         strings.TrimSpace(config.cf.QMgrName),
						"platform":     platformString,
						"subid":        subId,
						"subscription": subName,
						"type":         subTypeString,
						"topic":        topicString}).Set(f)
				}
			}
		}

		for _, attr := range e.qMgrStatus.Attributes {
			for _, value := range attr.Values {
				if value.IsInt64 && !attr.Pseudo {
					g := qMgrStatusGaugeMap[attr.MetricName]
					f := mqmetric.QueueManagerNormalise(attr, value.ValueInt64)

					g.With(prometheus.Labels{
						"qmgr":     strings.TrimSpace(config.cf.QMgrName),
						"platform": platformString}).Set(f)
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
						g := usageBpStatusGaugeMap[attr.MetricName]
						f := mqmetric.UsageNormalise(attr, value.ValueInt64)

						g.With(prometheus.Labels{
							"bufferpool": bpId,
							"location":   bpLocation,
							"pageclass":  bpClass,
							"qmgr":       strings.TrimSpace(config.cf.QMgrName),
							"platform":   platformString}).Set(f)
					}
				}
			}

			for _, attr := range e.usagePsStatus.Attributes {
				for key, value := range attr.Values {
					psId := e.usagePsStatus.Attributes[mqmetric.ATTR_PS_ID].Values[key].ValueString
					bpId := e.usagePsStatus.Attributes[mqmetric.ATTR_PS_BPID].Values[key].ValueString
					if value.IsInt64 && !attr.Pseudo {
						g := usagePsStatusGaugeMap[attr.MetricName]
						f := mqmetric.UsageNormalise(attr, value.ValueInt64)

						g.With(prometheus.Labels{
							"pageset":    psId,
							"bufferpool": bpId,
							"qmgr":       strings.TrimSpace(config.cf.QMgrName),
							"platform":   platformString}).Set(f)
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
			g := channelStatusGaugeMap[attr.MetricName]
			log.Debugf("Reporting chanl gauge for %s", attr.MetricName)
			g.Collect(ch)
		}
	}
	for _, attr := range e.qStatus.Attributes {
		if !attr.Pseudo {
			g := qStatusGaugeMap[attr.MetricName]
			log.Debugf("Reporting queue gauge for %s", attr.MetricName)
			g.Collect(ch)
		}
	}
	for _, attr := range e.topicStatus.Attributes {
		if !attr.Pseudo {
			g := topicStatusGaugeMap[attr.MetricName]
			//log.Debugf("Reporting topic gauge for %s", attr.MetricName)
			g.Collect(ch)
		}
	}
	for _, attr := range e.subStatus.Attributes {
		if !attr.Pseudo {
			g := subStatusGaugeMap[attr.MetricName]
			log.Debugf("Reporting subscription gauge for %s", attr.MetricName)
			g.Collect(ch)
		}
	}
	for _, attr := range e.qMgrStatus.Attributes {
		if !attr.Pseudo {
			g := qMgrStatusGaugeMap[attr.MetricName]
			log.Debugf("Reporting qmgr  gauge for %s", attr.MetricName)
			g.Collect(ch)
		}
	}

	if mqmetric.GetPlatform() == ibmmq.MQPL_ZOS {
		for _, attr := range e.usageBpStatus.Attributes {
			if !attr.Pseudo {
				g := usageBpStatusGaugeMap[attr.MetricName]
				log.Debugf("Reporting BPool gauge for %s", attr.MetricName)
				g.Collect(ch)
			}
		}
		for _, attr := range e.usagePsStatus.Attributes {
			if !attr.Pseudo {
				g := usagePsStatusGaugeMap[attr.MetricName]
				log.Debugf("Reporting Pageset gauge for %s", attr.MetricName)
				g.Collect(ch)
			}
		}
	}

	collectStopTime := time.Now()
	elapsedSecs := int64(collectStopTime.Sub(collectStartTime).Seconds())
	log.Infof("Collection time = %d secs", elapsedSecs)
	if elapsedSecs > defaultScrapeTimeout && !warnedScrapeTimeout {
		log.Warnf("Collection time has exceeded Prometheus default scrape_timeout value of %d seconds. Ensure you have set a larger value for this job", defaultScrapeTimeout)
		warnedScrapeTimeout = true
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
	if mqmetric.GetPlatform() == ibmmq.MQPL_ZOS {
		allocateUsageStatusGauges()
		log.Debugf("BP/PS  Gauges allocated")
	}
}

/*
allocateGauges creates a Prometheus gauge for each
resource that we know about. These are stored in a local map keyed
from the resource names.
*/
func allocateGauges() {
	for _, cl := range mqmetric.Metrics.Classes {
		for _, ty := range cl.Types {
			for _, elem := range ty.Elements {
				g := newMqGaugeVec(elem)
				key := makeKey(elem)
				gaugeMap[key] = g
			}
		}
	}
}

func allocateChannelStatusGauges() {
	// These attributes do not (currently) have an NLS translated description
	mqmetric.ChannelInitAttributes()
	for _, attr := range mqmetric.ChannelStatus.Attributes {
		description := attr.Description
		g := newMqGaugeVecObj(attr.MetricName, description, "channel")
		channelStatusGaugeMap[attr.MetricName] = g
	}
}

func allocateQStatusGauges() {
	mqmetric.QueueInitAttributes()
	for _, attr := range mqmetric.QueueStatus.Attributes {
		description := attr.Description
		g := newMqGaugeVecObj(attr.MetricName, description, "queue")
		qStatusGaugeMap[attr.MetricName] = g
	}
}

func allocateTopicStatusGauges() {
	mqmetric.TopicInitAttributes()
	for _, attr := range mqmetric.TopicStatus.Attributes {
		description := attr.Description
		g := newMqGaugeVecObj(attr.MetricName, description, "topic")
		topicStatusGaugeMap[attr.MetricName] = g
	}
}

func allocateSubStatusGauges() {
	mqmetric.SubInitAttributes()
	for _, attr := range mqmetric.SubStatus.Attributes {
		description := attr.Description
		g := newMqGaugeVecObj(attr.MetricName, description, "subscription")
		subStatusGaugeMap[attr.MetricName] = g
	}
}

func allocateQMgrStatusGauges() {
	mqmetric.QueueManagerInitAttributes()
	for _, attr := range mqmetric.QueueManagerStatus.Attributes {
		description := attr.Description
		g := newMqGaugeVecObj(attr.MetricName, description, "qmgr")
		qMgrStatusGaugeMap[attr.MetricName] = g
	}
}

func allocateUsageStatusGauges() {
	mqmetric.UsageInitAttributes()
	for _, attr := range mqmetric.UsageBpStatus.Attributes {
		description := attr.Description
		g := newMqGaugeVecObj(attr.MetricName, description, "bufferpool")
		usageBpStatusGaugeMap[attr.MetricName] = g
	}
	for _, attr := range mqmetric.UsagePsStatus.Attributes {
		description := attr.Description
		g := newMqGaugeVecObj(attr.MetricName, description, "pageset")
		usagePsStatusGaugeMap[attr.MetricName] = g
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
newMqGaugeVec returns the structure which will contain the
values and suitable labels. For queues we tag each entry
with both the queue and qmgr name; for the qmgr-wide entries, we
only need the single label.
*/
func newMqGaugeVec(elem *mqmetric.MonElement) *prometheus.GaugeVec {
	queueLabelNames := []string{"queue", "qmgr", "platform", "usage", "description"}
	qmgrLabelNames := []string{"qmgr", "platform"}

	labels := qmgrLabelNames
	prefix := "qmgr_"

	if strings.Contains(elem.Parent.ObjectTopic, "%s") {
		labels = queueLabelNames
		prefix = "queue_"
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
	gaugeVec := prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: config.namespace,
			Name:      prefix + elem.MetricName,
			Help:      description,
		},
		labels,
	)

	log.Debugf("Created gauge for '%s%s' from '%s'", prefix, elem.MetricName, elem.Description)
	return gaugeVec
}

/*
 * Create gauges for other object types. The status for these gauges is obtained via
 * a polling mechanism rather than pub/sub.
 * Only type in here for now is channels. But queues and topics may be suitable later
 */
func newMqGaugeVecObj(name string, description string, objectType string) *prometheus.GaugeVec {
	var labels []string

	prefix := objectType + "_"

	// There can be several channels active of the same name. They can be independently
	// identified by the MCAJobName attribute along with connName. So those are set as labels
	// on the gauge. The remote qmgr is also useful information to know.
	channelLabels := []string{"qmgr", "platform", objectType, "description",
		mqmetric.ATTR_CHL_TYPE,
		mqmetric.ATTR_CHL_RQMNAME,
		mqmetric.ATTR_CHL_CONNNAME,
		mqmetric.ATTR_CHL_JOBNAME}

	qmgrLabels := []string{"qmgr", "platform"}

	// With topic status, need to know if type is "pub" or "sub"
	topicLabels := []string{"qmgr", "platform", objectType, "type"}
	subLabels := []string{"qmgr", "platform", objectType, "subid", "topic", "type"}
	bpLabels := []string{"qmgr", "platform", objectType, "location", "pageclass"}
	psLabels := []string{"qmgr", "platform", objectType, "bufferpool"}
	// Adding the polling queue status options means we can use this block for
	// additional attributes. They should have the same labels as the stats generated
	// through resource publications.
	queueLabels := []string{"qmgr", "platform", objectType, "usage", "description"}

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
	default:
		log.Errorf("Tried to create gauge for unknown object type %s", objectType)
	}
	gaugeVec := prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: config.namespace,
			Name:      prefix + name,
			Help:      description,
		},
		labels,
	)

	log.Debugf("Created gauge for '%s%s' ", prefix, name)
	return gaugeVec
}
