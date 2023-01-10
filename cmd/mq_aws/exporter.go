package main

/*
  Copyright (c) IBM Corporation 2016,2022

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
This file pushes collected data to the Amazon CloudWatch service.
The Collect() function is the key operation
invoked at the configured intervals, causing us to read available publications
and update the various data points.
*/

import (
	"strings"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/cloudwatch"

	"github.com/ibm-messaging/mq-golang/v5/ibmmq"
	"github.com/ibm-messaging/mq-golang/v5/mqmetric"

	log "github.com/sirupsen/logrus"
)

type client struct {
	sess *session.Session
	svc  *cloudwatch.CloudWatch
}

var (
	first          = true
	errorCount     = 0
	c              client
	platformString = ""
	forceFlush     = false

	lastPoll           = time.Now()
	lastQueueDiscovery time.Time
)

/*
Collect is called by the main routine at regular intervals to provide current
data
*/
func Collect() error {
	var err error
	var series string
	log.Debugf("IBM MQ AWS collection started")
	collectStartTime := time.Now()

	if platformString == "" {
		platformString = strings.Replace(ibmmq.MQItoString("PL", int(mqmetric.GetPlatform())), "MQPL_", "", -1)
		log.Infof("Platform is %s", platformString)
	}

	if c.sess == nil {
		c.sess, err = session.NewSession()
		if err != nil {
			log.Fatal("Cannot create session: ", err)
		}
		c.svc = nil
	}

	if c.svc == nil {
		if config.ci.Region == "" {
			c.svc = cloudwatch.New(c.sess, aws.NewConfig())
		} else {
			c.svc = cloudwatch.New(c.sess, aws.NewConfig().WithRegion(config.ci.Region))
		}
		if err != nil {
			log.Fatal("Cannot create service: ", err)
		}
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
	} else {
		log.Debugf("Skipping poll for object status")
	}

	pollError := err
	if pollStatus {
		if config.cf.CC.UseStatus {
			err := mqmetric.CollectChannelStatus(config.cf.MonitoredChannels)
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
		}

		err = mqmetric.CollectQueueManagerStatus()
		if err != nil {
			log.Errorf("Error collecting queue manager status: %v", err)
			pollError = err
		} else {
			log.Debugf("Collected all queue manager status")
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

			err = pollError
		}

		if err != nil {
			log.Fatalf("Error collecting status: %v", err)
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
				err = mqmetric.RediscoverAttributes(mqmetric.OT_CHANNEL_AMQP, config.cf.MonitoredAMQPChannels)

				//}
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
			bp := newBatchPoints()

			// Start with a metric that shows how many publications were processed by this collection
			series = "qmgr"
			tags := map[string]string{
				"qmgr":     config.cf.QMgrName,
				"platform": platformString,
			}
			pt, _ := newPoint(series+"."+"exporter_publications", t, float64(mqmetric.GetProcessPublicationCount()), tags)
			bp.addPoint(pt)
			log.Debugf("Adding point %v", pt)

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

							pt, _ := newPoint(series+"."+elem.MetricName, t, float64(f), tags)
							bp.addPoint(pt)

							log.Debugf("Adding point %v", pt)

							// AWS recommends not sending too many
							// data points in a single request.
							// So we flush after a configurable set has been
							// collected.
							bp = c.Flush(bp)

						}
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

						tags := map[string]string{
							"qmgr": config.cf.QMgrName,
						}
						tags["channel"] = chlName
						tags["platform"] = platformString
						tags[mqmetric.ATTR_CHL_TYPE] = strings.TrimSpace(chlTypeString)
						tags[mqmetric.ATTR_CHL_RQMNAME] = strings.TrimSpace(rqmName)
						tags[mqmetric.ATTR_CHL_CONNNAME] = strings.TrimSpace(connName)
						tags[mqmetric.ATTR_CHL_JOBNAME] = strings.TrimSpace(jobName)

						f := mqmetric.ChannelNormalise(attr, value.ValueInt64)

						pt, _ := newPoint(series+"."+attr.MetricName, t, float64(f), tags)
						bp.addPoint(pt)
						bp = c.Flush(bp)
						log.Debugf("Adding channel point %v", pt)
					}
				}
			}

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

						f := mqmetric.QueueNormalise(attr, value.ValueInt64)

						pt, _ := newPoint(series+"."+attr.MetricName, t, float64(f), tags)

						bp.addPoint(pt)
						bp = c.Flush(bp)

						log.Debugf("Adding queue point %v", pt)
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

						f := mqmetric.TopicNormalise(attr, value.ValueInt64)

						pt, _ := newPoint(series+"."+attr.MetricName, t, float64(f), tags)

						bp.addPoint(pt)
						bp = c.Flush(bp)

						log.Debugf("Adding topic point %v", pt)
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
						f := mqmetric.SubNormalise(attr, value.ValueInt64)

						pt, _ := newPoint(series+"."+attr.MetricName, t, float64(f), tags)

						bp.addPoint(pt)
						bp = c.Flush(bp)

						log.Debugf("Adding subscription point %v", pt)
					}
				}
			}

			series = "cluster"
			for _, attr := range mqmetric.GetObjectStatus("", mqmetric.OT_CLUSTER).Attributes {
				for key, value := range attr.Values {
					if value.IsInt64 {

						qMgrName := strings.TrimSpace(config.cf.QMgrName)
						clusterName := mqmetric.GetObjectStatus("", mqmetric.OT_CLUSTER).Attributes[mqmetric.ATTR_CLUSTER_NAME].Values[key].ValueString
						qmType := mqmetric.GetObjectStatus("", mqmetric.OT_CLUSTER).Attributes[mqmetric.ATTR_CLUSTER_QMTYPE].Values[key].ValueInt64

						qmTypeString := "PARTIAL"
						if qmType == int64(ibmmq.MQQMT_REPOSITORY) {
							qmTypeString = "FULL"
						}
						tags := map[string]string{
							"qmgr":     qMgrName,
							"platform": platformString,
							"cluster":  clusterName,
							"qmtype":   qmTypeString,
						}

						f := mqmetric.ClusterNormalise(attr, value.ValueInt64)

						pt, _ := newPoint(series+"."+attr.MetricName, t, float64(f), tags)

						bp.addPoint(pt)
						bp = c.Flush(bp)

						log.Debugf("Adding cluster point %v", pt)
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

						f := mqmetric.QueueManagerNormalise(attr, value.ValueInt64)

						pt, _ := newPoint(series+"."+attr.MetricName, t, float64(f), tags)

						bp.addPoint(pt)
						bp = c.Flush(bp)

						log.Debugf("Adding qmgr point %v", pt)
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

							f := mqmetric.UsageNormalise(attr, value.ValueInt64)

							pt, _ := newPoint(series+"."+attr.MetricName, t, float64(f), tags)
							bp.addPoint(pt)
							bp = c.Flush(bp)

							log.Debugf("Adding bufferpool point %v", pt)

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
							f := mqmetric.UsageNormalise(attr, value.ValueInt64)

							pt, _ := newPoint(series+"."+attr.MetricName, t, float64(f), tags)
							bp.addPoint(pt)
							bp = c.Flush(bp)
							log.Debugf("Adding pageset point %v", pt)

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

							f := mqmetric.ChannelNormalise(attr, value.ValueInt64)

							pt, _ := newPoint(series+"."+attr.MetricName, t, float64(f), tags)
							bp.addPoint(pt)
							bp = c.Flush(bp)
							log.Debugf("Adding AMQP point %v", pt)

						}
					}
				}
			}

			forceFlush = true
			c.Flush(bp)
		}
	}

	collectStopTime := time.Now()
	elapsedSecs := int64(collectStopTime.Sub(collectStartTime).Seconds())
	log.Debugf("Collection time = %d secs", elapsedSecs)

	return err

}

func (c client) Flush(bp *BatchPoints) *BatchPoints {
	// This is where real errors might occur, including the inability to
	// contact the server. We will ignore (but log) these errors
	// up to a threshold, after which it is considered fatal.
	if len(bp.Points) < config.ci.MaxPoints && !forceFlush {
		return bp
	}

	if len(bp.Points) > 0 {
		forceFlush = false
		err := c.Put(bp)
		if err != nil {
			log.Error(err)
			errorCount++
			if errorCount >= config.ci.MaxErrors {
				log.Fatal("Too many errors communicating with server")
			}
		} else {
			errorCount = 0
		}
	}
	bp = newBatchPoints()
	return bp
}

func (c client) Put(bp *BatchPoints) error {
	if len(bp.Points) == 0 {
		return nil
	}

	log.Debugf("Serialised points are %+v", bp)
	log.Infof("Putting %d points", len(bp.Points))
	params := &cloudwatch.PutMetricDataInput{
		MetricData: bp.Points,
		Namespace:  aws.String(config.ci.Namespace),
	}

	_, err := c.svc.PutMetricData(params)
	return err
}
