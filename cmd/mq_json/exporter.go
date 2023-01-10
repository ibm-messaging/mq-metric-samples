package main

/*
  Copyright (c) IBM Corporation 2016, 2022

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
The Collect() function is the key operation
invoked at the configured intervals, causing us to read available publications
and update the various data points.
*/

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/ibm-messaging/mq-golang/v5/ibmmq"
	"github.com/ibm-messaging/mq-golang/v5/mqmetric"
	log "github.com/sirupsen/logrus"
)

var (
	first              = true
	errorCount         = 0
	lastPoll           = time.Now()
	lastQueueDiscovery = time.Now()
	platformString     = ""
	fixupString        = make(map[string]string)
)

const (
	blankString = "                                "
)

type collectionTimeStruct struct {
	TimeStamp string `json:"timeStamp"`
	Epoch     int64  `json:"epoch"`
}

type pointsStruct struct {
	ObjectType string             `json:"objectType"`
	Tags       map[string]string  `json:"tags"`
	Metric     map[string]float64 `json:"metrics"`
}

type jsonReportStruct struct {
	CollectionTime collectionTimeStruct `json:"collectionTime"`
	Points         []pointsStruct       `json:"points"`
}

var AllPoints []pointsStruct

/*
Collect is called by the main routine at regular intervals to provide current
data
*/
func Collect() error {
	var err error
	var j jsonReportStruct

	log.Debugf("IBM MQ JSON collector started")
	collectStartTime := time.Now()

	if platformString == "" {
		platformString = strings.Replace(ibmmq.MQItoString("PL", int(mqmetric.GetPlatform())), "MQPL_", "", -1)
	}
	// Clear out everything we know so far. In particular, replace
	// the map of values for each object so the collection starts
	// clean.
	for _, cl := range mqmetric.GetPublishedMetrics("").Classes {
		for _, ty := range cl.Types {
			for _, elem := range ty.Elements {
				elem.Values = make(map[string]int64)
			}
		}
	}

	// Process all the publications that have arrived
	err = mqmetric.ProcessPublications()
	if err != nil {
		log.Fatalf("Error processing publications: %v", err)
	}

	// Do we need to poll for object status on this iteration
	pollStatus := false
	thisPoll := time.Now()
	elapsed := thisPoll.Sub(lastPoll)
	if elapsed >= config.cf.PollIntervalDuration || first {
		log.Debugf("Polling for object status")
		lastPoll = thisPoll
		pollStatus = true
	}

	// If there has been sufficient interval since the last explicit poll for
	// status, then do that collection too
	pollError := err

	if pollStatus {
		if config.cf.CC.UseStatus {
			err := mqmetric.CollectQueueManagerStatus()
			if err != nil {
				log.Errorf("Error collecting queue manager status: %v", err)
				pollError = err
			} else {
				log.Debugf("Collected all queue manager status")
			}
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
			}
		}
		err = pollError
	}

	if err != nil {
		log.Fatalf("Error collecting status: %v", err)
	}

	thisDiscovery := time.Now()
	elapsed = thisDiscovery.Sub(lastQueueDiscovery)
	if config.cf.RediscoverDuration > 0 {
		if elapsed >= config.cf.RediscoverDuration {
			err = mqmetric.RediscoverAndSubscribe(discoverConfig)
			lastQueueDiscovery = thisDiscovery
			err = mqmetric.RediscoverAttributes(ibmmq.MQOT_CHANNEL, config.cf.MonitoredChannels)
			err = mqmetric.RediscoverAttributes(mqmetric.OT_CHANNEL_AMQP, config.cf.MonitoredAMQPChannels)
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

		t := time.Now()
		j.CollectionTime.TimeStamp = t.Format(time.RFC3339)
		j.CollectionTime.Epoch = t.Unix()

		// All of the metrics for a given set of tags are printed in a single
		// JSON object.
		ptMapPub := make(map[string]pointsStruct)
		var pt pointsStruct
		var ok bool

		for _, cl := range mqmetric.GetPublishedMetrics("").Classes {
			for _, ty := range cl.Types {
				for _, elem := range ty.Elements {
					for key, value := range elem.Values {

						//log.Debugf("Proccesing published metrics for key %s", key)
						if pt, ok = ptMapPub[key]; !ok {
							pt = pointsStruct{}
							pt.Tags = make(map[string]string)
							pt.Metric = make(map[string]float64)

							pt.Tags["qmgr"] = config.cf.QMgrName
							pt.ObjectType = "qmgr"
							pt.Tags["platform"] = platformString
							if key == mqmetric.QMgrMapKey {
								pt.Tags["description"] = mqmetric.GetObjectDescription("", ibmmq.MQOT_Q_MGR)
							} else {
								usageString := getUsageString(key)
								pt.Tags["queue"] = key
								pt.Tags["usage"] = usageString
								pt.ObjectType = "queue"
								pt.Tags["description"] = mqmetric.GetObjectDescription(key, ibmmq.MQOT_Q)
								pt.Tags["cluster"] = mqmetric.GetQueueAttribute(key, ibmmq.MQCA_CLUSTER_NAME)

							}
						}

						pt.Metric[fixup(elem.MetricName)] = mqmetric.Normalise(elem, key, value)
						ptMapPub[key] = pt
					}
				}
			}
		}

		// Add a metric that shows how many publications were processed by this collection
		key := mqmetric.QMgrMapKey
		if pt, ok = ptMapPub[key]; ok {
			pt = ptMapPub[key]
			pt.Metric[fixup("exporter_publications")] = float64(mqmetric.GetProcessPublicationCount())
			ptMapPub[key] = pt
		}

		// Next we extract the info for channel status. Several of the attributes
		// are used to build the tags that uniquely identify a channel instance.
		if pollStatus {
			ptMap := make(map[string]pointsStruct)

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
						key1 := "channel/" + chlName + "/" + connName + "/" + jobName + "/" + rqmName

						if pt, ok = ptMap[key1]; !ok {
							pt = pointsStruct{}
							pt.ObjectType = "channel"
							pt.Tags = make(map[string]string)
							pt.Metric = make(map[string]float64)

							pt.Tags["qmgr"] = strings.TrimSpace(config.cf.QMgrName)
							pt.Tags["channel"] = chlName
							pt.Tags["platform"] = platformString
							pt.Tags["description"] = mqmetric.GetObjectDescription(chlName, ibmmq.MQOT_CHANNEL)
							pt.Tags[mqmetric.ATTR_CHL_TYPE] = strings.TrimSpace(chlTypeString)
							pt.Tags[mqmetric.ATTR_CHL_RQMNAME] = strings.TrimSpace(rqmName)
							pt.Tags[mqmetric.ATTR_CHL_CONNNAME] = strings.TrimSpace(connName)
							pt.Tags[mqmetric.ATTR_CHL_JOBNAME] = strings.TrimSpace(jobName)

						}
						pt.Metric[fixup(attr.MetricName)] = mqmetric.ChannelNormalise(attr, value.ValueInt64)
						ptMap[key1] = pt
					}
				}

				for _, attr := range mqmetric.GetObjectStatus("", mqmetric.OT_Q).Attributes {
					for key, value := range attr.Values {
						if value.IsInt64 {
							qName := mqmetric.GetObjectStatus("", mqmetric.OT_Q).Attributes[mqmetric.ATTR_Q_NAME].Values[key].ValueString
							usageString := getUsageString(key)

							key1 := "queue/" + qName

							if pt, ok = ptMapPub[qName]; !ok {
								if pt, ok = ptMap[key1]; !ok {
									pt = pointsStruct{}
									pt.ObjectType = "queue"
									pt.Metric = make(map[string]float64)
									pt.Tags = make(map[string]string)
									pt.Tags["qmgr"] = strings.TrimSpace(config.cf.QMgrName)
									pt.Tags["queue"] = qName
									pt.Tags["usage"] = usageString
									pt.Tags["description"] = mqmetric.GetObjectDescription(qName, ibmmq.MQOT_Q)
									pt.Tags["cluster"] = mqmetric.GetQueueAttribute(key, ibmmq.MQCA_CLUSTER_NAME)
									pt.Tags["platform"] = platformString
								}
							}

							pt.Metric[fixup(attr.MetricName)] = mqmetric.QueueNormalise(attr, value.ValueInt64)
							ptMap[key1] = pt
						}
					}
				}

				for _, attr := range mqmetric.GetObjectStatus("", mqmetric.OT_TOPIC).Attributes {
					for key, value := range attr.Values {
						if value.IsInt64 {
							topicName := mqmetric.GetObjectStatus("", mqmetric.OT_TOPIC).Attributes[mqmetric.ATTR_TOPIC_STRING].Values[key].ValueString
							topicStatusType := mqmetric.GetObjectStatus("", mqmetric.OT_TOPIC).Attributes[mqmetric.ATTR_TOPIC_STATUS_TYPE].Values[key].ValueString
							key1 := "topic/" + mqmetric.TopicKey(topicName, topicStatusType)

							if pt, ok = ptMap[key1]; !ok {
								pt = pointsStruct{}
								pt.ObjectType = "topic"
								pt.Metric = make(map[string]float64)
								pt.Tags = make(map[string]string)
								pt.Tags["qmgr"] = strings.TrimSpace(config.cf.QMgrName)
								pt.Tags["topic"] = topicName
								pt.Tags["platform"] = platformString
								pt.Tags["type"] = topicStatusType
							}

							pt.Metric[fixup(attr.MetricName)] = mqmetric.TopicNormalise(attr, value.ValueInt64)
							ptMap[key1] = pt
						}
					}
				}

				for _, attr := range mqmetric.GetObjectStatus("", mqmetric.OT_Q_MGR).Attributes {
					for key, value := range attr.Values {
						if value.IsInt64 {
							qMgrName := mqmetric.GetObjectStatus("", mqmetric.OT_Q_MGR).Attributes[mqmetric.ATTR_QMGR_NAME].Values[key].ValueString

							key1 := "qmgr/" + qMgrName

							if pt, ok = ptMapPub[mqmetric.QMgrMapKey]; !ok {
								if pt, ok = ptMap[key1]; !ok {
									pt = pointsStruct{}
									pt.ObjectType = "qmgr"
									pt.Metric = make(map[string]float64)
									pt.Tags = make(map[string]string)
									pt.Tags["qmgr"] = strings.TrimSpace(qMgrName)
									pt.Tags["platform"] = platformString
									pt.Tags["description"] = mqmetric.GetObjectDescription("", ibmmq.MQOT_Q_MGR)
								}
							}
							pt.Metric[fixup(attr.MetricName)] = mqmetric.QueueManagerNormalise(attr, value.ValueInt64)
							ptMap[key1] = pt
						}
					}
				}

				for _, attr := range mqmetric.GetObjectStatus("", mqmetric.OT_SUB).Attributes {
					for key, value := range attr.Values {
						if value.IsInt64 {
							subId := mqmetric.GetObjectStatus("", mqmetric.OT_SUB).Attributes[mqmetric.ATTR_SUB_ID].Values[key].ValueString
							subName := mqmetric.GetObjectStatus("", mqmetric.OT_SUB).Attributes[mqmetric.ATTR_SUB_NAME].Values[key].ValueString
							subType := int(mqmetric.GetObjectStatus("", mqmetric.OT_SUB).Attributes[mqmetric.ATTR_SUB_TYPE].Values[key].ValueInt64)
							subTypeString := strings.Replace(ibmmq.MQItoString("SUBTYPE", subType), "MQSUBTYPE_", "", -1)
							topicString := mqmetric.GetObjectStatus("", mqmetric.OT_SUB).Attributes[mqmetric.ATTR_SUB_TOPIC_STRING].Values[key].ValueString

							key1 := "subscription/" + subId

							if pt, ok = ptMap[key1]; !ok {
								pt = pointsStruct{}
								pt.ObjectType = "subscription"
								pt.Metric = make(map[string]float64)
								pt.Tags = make(map[string]string)
								pt.Tags["qmgr"] = strings.TrimSpace(config.cf.QMgrName)
								pt.Tags["platform"] = platformString
								pt.Tags["subid"] = subId
								pt.Tags["subscription"] = subName
								pt.Tags["type"] = subTypeString
								pt.Tags["topic"] = topicString
							}

							pt.Metric[fixup(attr.MetricName)] = mqmetric.SubNormalise(attr, value.ValueInt64)
							ptMap[key1] = pt
						}
					}
				}

				for _, attr := range mqmetric.GetObjectStatus("", mqmetric.OT_CLUSTER).Attributes {
					for key, value := range attr.Values {
						if value.IsInt64 {
							clusterName := mqmetric.GetObjectStatus("", mqmetric.OT_CLUSTER).Attributes[mqmetric.ATTR_CLUSTER_NAME].Values[key].ValueString
							qmType := mqmetric.GetObjectStatus("", mqmetric.OT_CLUSTER).Attributes[mqmetric.ATTR_CLUSTER_QMTYPE].Values[key].ValueInt64

							qmTypeString := "PARTIAL"
							if qmType == int64(ibmmq.MQQMT_REPOSITORY) {
								qmTypeString = "FULL"
							}

							key1 := "cluster/" + clusterName

							if pt, ok = ptMap[key1]; !ok {
								pt = pointsStruct{}
								pt.ObjectType = "subscription"
								pt.Metric = make(map[string]float64)
								pt.Tags = make(map[string]string)
								pt.Tags["qmgr"] = strings.TrimSpace(config.cf.QMgrName)
								pt.Tags["platform"] = platformString

								pt.Tags["qmtype"] = qmTypeString
								pt.Tags["cluster"] = clusterName
							}

							pt.Metric[fixup(attr.MetricName)] = mqmetric.ClusterNormalise(attr, value.ValueInt64)
							ptMap[key1] = pt
						}
					}
				}

				if mqmetric.GetPlatform() == ibmmq.MQPL_ZOS {
					for _, attr := range mqmetric.GetObjectStatus("", mqmetric.OT_BP).Attributes {
						for key, value := range attr.Values {
							bpId := mqmetric.GetObjectStatus("", mqmetric.OT_BP).Attributes[mqmetric.ATTR_BP_ID].Values[key].ValueString
							bpLocation := mqmetric.GetObjectStatus("", mqmetric.OT_BP).Attributes[mqmetric.ATTR_BP_LOCATION].Values[key].ValueString
							bpClass := mqmetric.GetObjectStatus("", mqmetric.OT_BP).Attributes[mqmetric.ATTR_BP_CLASS].Values[key].ValueString
							if value.IsInt64 && !attr.Pseudo {
								key1 := "bufferpool/" + bpId
								if pt, ok = ptMap[key1]; !ok {
									pt = pointsStruct{}
									pt.ObjectType = "bufferpool"
									pt.Metric = make(map[string]float64)
									pt.Tags = make(map[string]string)
									pt.Tags["bufferpool"] = bpId
									pt.Tags["location"] = bpLocation
									pt.Tags["pageclass"] = bpClass
									pt.Tags["qmgr"] = strings.TrimSpace(config.cf.QMgrName)
									pt.Tags["platform"] = platformString
								}
								pt.Metric[fixup(attr.MetricName)] = mqmetric.UsageNormalise(attr, value.ValueInt64)
								ptMap[key1] = pt
							}
						}
					}

					for _, attr := range mqmetric.GetObjectStatus("", mqmetric.OT_PS).Attributes {
						for key, value := range attr.Values {
							psId := mqmetric.GetObjectStatus("", mqmetric.OT_PS).Attributes[mqmetric.ATTR_PS_ID].Values[key].ValueString
							bpId := mqmetric.GetObjectStatus("", mqmetric.OT_PS).Attributes[mqmetric.ATTR_PS_BPID].Values[key].ValueString
							if value.IsInt64 && !attr.Pseudo {
								key1 := "pageset/" + psId
								if pt, ok = ptMap[key1]; !ok {
									pt = pointsStruct{}
									pt.ObjectType = "pageset"
									pt.Metric = make(map[string]float64)
									pt.Tags = make(map[string]string)
									pt.Tags["pageset"] = psId
									pt.Tags["bufferpool"] = bpId
									pt.Tags["qmgr"] = strings.TrimSpace(config.cf.QMgrName)
									pt.Tags["platform"] = platformString
								}
								pt.Metric[fixup(attr.MetricName)] = mqmetric.UsageNormalise(attr, value.ValueInt64)
								ptMap[key1] = pt
							}

						}
					}
				} else {
					for _, attr := range mqmetric.GetObjectStatus("", mqmetric.OT_CHANNEL_AMQP).Attributes {
						for key, value := range attr.Values {
							chlName := mqmetric.GetObjectStatus("", mqmetric.OT_CHANNEL_AMQP).Attributes[mqmetric.ATTR_CHL_NAME].Values[key].ValueString
							clientId := mqmetric.GetObjectStatus("", mqmetric.OT_CHANNEL_AMQP).Attributes[mqmetric.ATTR_CHL_AMQP_CLIENT_ID].Values[key].ValueString
							connName := mqmetric.GetObjectStatus("", mqmetric.OT_CHANNEL_AMQP).Attributes[mqmetric.ATTR_CHL_CONNNAME].Values[key].ValueString
							if value.IsInt64 && !attr.Pseudo {
								key1 := "amqp/" + chlName + "/" + connName + "/" + clientId
								if pt, ok = ptMap[key1]; !ok {
									pt = pointsStruct{}
									pt.ObjectType = "amqp"
									pt.Metric = make(map[string]float64)
									pt.Tags = make(map[string]string)
									pt.Tags["qmgr"] = strings.TrimSpace(config.cf.QMgrName)
									pt.Tags["channel"] = chlName
									pt.Tags["description"] = mqmetric.GetObjectDescription(chlName, mqmetric.OT_CHANNEL_AMQP)
									pt.Tags["platform"] = platformString
									pt.Tags[mqmetric.ATTR_CHL_CONNNAME] = strings.TrimSpace(connName)
									pt.Tags[mqmetric.ATTR_CHL_AMQP_CLIENT_ID] = clientId
								}
								pt.Metric[fixup(attr.MetricName)] = mqmetric.ChannelNormalise(attr, value.ValueInt64)
								ptMap[key1] = pt
							}
						}
					}
				}

			}

			// Make sure we start with an empty array, and then add the xxSTATUS metrics
			AllPoints = nil
			for _, pt := range ptMap {
				AllPoints = append(AllPoints, pt)
			}
		}

		// Now add the published metrics, which might have some of the xxSTATUS metrics merged
		for _, pt := range ptMapPub {
			AllPoints = append(AllPoints, pt)
		}

		// Finally split the records, if requested, so that each block is not TOO long
		for _, chunk := range chunk(AllPoints, config.recordmax) {
			j.Points = chunk
			if config.oneline {
				b, _ := json.Marshal(j)
				fmt.Printf("%s\n", b)
			} else {
				b, _ := json.MarshalIndent(j, "", "  ")
				fmt.Printf("%s\n", b)
			}
		}

	}

	collectStopTime := time.Now()
	elapsedSecs := int64(collectStopTime.Sub(collectStartTime).Seconds())
	log.Debugf("Collection time = %d secs", elapsedSecs)

	return err

}

func getUsageString(key string) string {
	usageString := ""
	if valuesMap, ok := mqmetric.GetObjectStatus("", mqmetric.OT_Q).Attributes[mqmetric.ATTR_Q_USAGE]; ok {
		if usage, ok := valuesMap.Values[key]; ok {
			if usage.ValueInt64 == int64(ibmmq.MQUS_TRANSMISSION) {
				usageString = "XMITQ"
			} else {
				usageString = "NORMAL"
			}
		}
	}
	return usageString
}

func fixup(s1 string) string {
	// Another reformatting of the metric name - this one converts
	// something like queue_avoided_bytes into queueAvoidedBytes

	// The new name is cached, so next time round we can find it immediately
	if s2, ok := fixupString[s1]; ok {
		return s2
	}

	s2 := ""
	c := ""
	nextCaseUpper := false

	for i := 0; i < len(s1); i++ {
		if s1[i] != '_' {
			if nextCaseUpper {
				c = strings.ToUpper(s1[i : i+1])
				nextCaseUpper = false
			} else {
				c = strings.ToLower(s1[i : i+1])
			}
			s2 += c
		} else {
			nextCaseUpper = true
		}

	}

	fixupString[s1] = s2
	return s2
}

// Split an array/slice into several chunks so that not all points are
// dumped in the same JSON array.
func chunk(slice []pointsStruct, chunkSize int) [][]pointsStruct {
	var chunks [][]pointsStruct

	if chunkSize <= 0 { // Allow the size to be unlimited: 0 & -1 both achieve that
		chunkSize = len(slice)
	}
	for i := 0; i < len(slice); i += chunkSize {
		end := i + chunkSize
		if end > len(slice) {
			end = len(slice)
		}
		chunks = append(chunks, slice[i:end])
	}
	return chunks
}
