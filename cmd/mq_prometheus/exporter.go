package main

/*
  Copyright (c) IBM Corporation 2016, 2018

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

	"github.com/ibm-messaging/mq-golang/ibmmq"
	"github.com/ibm-messaging/mq-golang/mqmetric"
	"github.com/prometheus/client_golang/prometheus"
	log "github.com/sirupsen/logrus"
)

type exporter struct {
	mutex     sync.RWMutex
	metrics   mqmetric.AllMetrics
	chlStatus mqmetric.StatusSet
	qStatus   mqmetric.StatusSet
}

func newExporter() *exporter {
	return &exporter{
		metrics:   mqmetric.Metrics,
		chlStatus: mqmetric.ChannelStatus,
		qStatus:   mqmetric.QueueStatus,
	}
}

var (
	first                 = true
	gaugeMap              = make(map[string]*prometheus.GaugeVec)
	channelStatusGaugeMap = make(map[string]*prometheus.GaugeVec)
	qStatusGaugeMap       = make(map[string]*prometheus.GaugeVec)
	lastPoll              = time.Now()
	platformString        string
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
}

/*
Collect is called by Prometheus at regular intervals to provide current
data
*/
func (e *exporter) Collect(ch chan<- prometheus.Metric) {
	e.mutex.Lock() // To protect metrics from concurrent collects.
	defer e.mutex.Unlock()

	log.Infof("IBMMQ Collect started")

	// Do we need to poll for object status on this iteration
	pollStatus := false
	thisPoll := time.Now()
	elapsed := thisPoll.Sub(lastPoll)
	if elapsed >= config.pollIntervalDuration {
		log.Debugf("Polling for object status")
		lastPoll = thisPoll
		pollStatus = true
	}

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
	mqmetric.ProcessPublications()

	// If there has been sufficient interval since the last explicit poll for
	// status, then do that collection too
	if pollStatus {
		for _, attr := range e.chlStatus.Attributes {
			channelStatusGaugeMap[attr.MetricName].Reset()
		}
		for _, attr := range e.qStatus.Attributes {
			qStatusGaugeMap[attr.MetricName].Reset()
		}

		err := mqmetric.CollectChannelStatus(config.monitoredChannels)
		if err != nil {
			log.Errorf("Error collecting channel status: %v", err)
		} else {
			log.Debugf("Collected all channel status")
		}

		if config.qStatus {
			err = mqmetric.CollectQueueStatus(config.monitoredQueues)
			if err != nil {
				log.Errorf("Error collecting queue status: %v", err)
			} else {
				log.Debugf("Collected all queue status")
			}
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
							g.With(prometheus.Labels{"qmgr": config.qMgrName,
								"platform": platformString}).Set(f)
						} else {
							g.With(prometheus.Labels{"qmgr": config.qMgrName,
								"queue":    key,
								"platform": platformString}).Set(f)
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

	// Next we extract the info for channel status. Several of the attributes
	// are used to build the tags that uniquely identify a channel instance
	if pollStatus {
		for _, attr := range e.chlStatus.Attributes {
			for key, value := range attr.Values {
				if value.IsInt64 {
					g := channelStatusGaugeMap[attr.MetricName]

					f := mqmetric.ChannelNormalise(attr, value.ValueInt64)

					chlType := int(e.chlStatus.Attributes[mqmetric.ATTR_CHL_TYPE].Values[key].ValueInt64)
					chlTypeString := strings.Replace(ibmmq.MQItoString("CHT", chlType), "MQCHT_", "", -1)
					// Not every channel status report has the RQMNAME attribute (eg SVRCONNs)
					rqmname := ""
					if rqmnameAttr, ok := e.chlStatus.Attributes[mqmetric.ATTR_CHL_RQMNAME].Values[key]; ok {
						rqmname = rqmnameAttr.ValueString
					}

					chlName := e.chlStatus.Attributes[mqmetric.ATTR_CHL_NAME].Values[key].ValueString
					connName := e.chlStatus.Attributes[mqmetric.ATTR_CHL_CONNNAME].Values[key].ValueString
					jobName := e.chlStatus.Attributes[mqmetric.ATTR_CHL_JOBNAME].Values[key].ValueString

					g.With(prometheus.Labels{
						"qmgr":                     strings.TrimSpace(config.qMgrName),
						"channel":                  chlName,
						"platform":                 platformString,
						mqmetric.ATTR_CHL_TYPE:     strings.TrimSpace(chlTypeString),
						mqmetric.ATTR_CHL_RQMNAME:  strings.TrimSpace(rqmname),
						mqmetric.ATTR_CHL_CONNNAME: strings.TrimSpace(connName),
						mqmetric.ATTR_CHL_JOBNAME:  strings.TrimSpace(jobName)}).Set(f)
				}
			}
		}

		for _, attr := range e.qStatus.Attributes {
			for key, value := range attr.Values {
				if value.IsInt64 {
					qName := e.qStatus.Attributes[mqmetric.ATTR_Q_NAME].Values[key].ValueString
					g := qStatusGaugeMap[attr.MetricName]
					f := mqmetric.QueueNormalise(attr, value.ValueInt64)

					g.With(prometheus.Labels{
						"qmgr":     strings.TrimSpace(config.qMgrName),
						"platform": platformString,
						"queue":    qName}).Set(f)
				}
			}
		}
	}

	// Then put the channel info back to Prometheus
	// We do this even if we have not polled for new status, so that Grafana's "instant"
	// view will still show up the most recently known values
	for _, attr := range e.chlStatus.Attributes {
		g := channelStatusGaugeMap[attr.MetricName]
		log.Debugf("Reporting gauge for %s", attr.MetricName)
		g.Collect(ch)
	}
	for _, attr := range e.qStatus.Attributes {
		g := qStatusGaugeMap[attr.MetricName]
		log.Debugf("Reporting gauge for %s", attr.MetricName)
		g.Collect(ch)
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
	queueLabelNames := []string{"queue", "qmgr", "platform"}
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

	log.Infof("Created gauge for '%s%s' from '%s'", prefix, elem.MetricName,elem.Description)
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
	channelLabels := []string{"qmgr", "platform", objectType,
		mqmetric.ATTR_CHL_TYPE,
		mqmetric.ATTR_CHL_RQMNAME,
		mqmetric.ATTR_CHL_CONNNAME,
		mqmetric.ATTR_CHL_JOBNAME}

	// Adding the polling queue status options means we can use this block for
	// additional attributes. They should have the same labels as the published stats
	genericLabels := []string{"qmgr", "platform", objectType}

	switch objectType {
	case "channel":
		labels = channelLabels
	case "queue":
		labels = genericLabels
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

	log.Infof("Created gauge for '%s%s' ", prefix, name )
	return gaugeVec
}
