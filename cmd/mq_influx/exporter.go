package main

/*
  Copyright (c) IBM Corporation 2016

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
This file pushes collected data to the InfluxDB.
The Collect() function is the key operation
invoked at the configured intervals, causing us to read available publications
and update the various data points.
*/

import (
	ibmmq "github.com/ibm-messaging/mq-golang/ibmmq"
	mqmetric "github.com/ibm-messaging/mq-golang/mqmetric"
	client "github.com/influxdata/influxdb1-client/v2"
	log "github.com/sirupsen/logrus"
	"strings"
	"time"
)

var (
	first          = true
	errorCount     = 0
	platformString = ""
)

/*
Collect is called by the main routine at regular intervals to provide current
data
*/
func Collect(c client.Client) error {
	var err error
	var series string

	log.Infof("IBMMQ InfluxDB collection started")

	if platformString == "" {
		platformString = strings.Replace(ibmmq.MQItoString("PL", int(mqmetric.GetPlatform())), "MQPL_", "", -1)
	}

	// Clear out everything we know so far. In particular, replace
	// the map of values for each object so the collection starts
	// clean.
	for _, cl := range mqmetric.Metrics.Classes {
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

	err = mqmetric.CollectQueueManagerStatus()
	if err != nil {
		log.Errorf("Error collecting queue manager status: %v", err)
	} else {
		log.Debugf("Collected all queue manager status")
	}
	err = mqmetric.CollectChannelStatus(config.cf.MonitoredChannels)
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
	if config.cf.CC.UseStatus {
		err = mqmetric.CollectQueueStatus(config.cf.MonitoredQueues)
		if err != nil {
			log.Errorf("Error collecting queue status: %v", err)
		} else {
			log.Debugf("Collected all queue status")
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
		bp, _ := client.NewBatchPoints(client.BatchPointsConfig{
			Database:  config.databaseName,
			Precision: "ms",
		})
		if err != nil {
			// This kind of error is so unlikely, it should be treated as fatal
			log.Fatalln("Error creating batch points: ", err)
		}

		for _, cl := range mqmetric.Metrics.Classes {
			for _, ty := range cl.Types {
				for _, elem := range ty.Elements {
					for key, value := range elem.Values {
						f := mqmetric.Normalise(elem, key, value)
						tags := map[string]string{
							"qmgr": config.cf.QMgrName,
						}

						series = "qmgr"
						if key != mqmetric.QMgrMapKey {
							tags["queue"] = key
							series = "queue"
						}
						fields := map[string]interface{}{elem.MetricName: f}
						pt, _ := client.NewPoint(series, tags, fields, t)
						bp.AddPoint(pt)
						log.Debugf("Adding point %v", pt)
					}
				}
			}

			series = "channel"
			for _, attr := range mqmetric.ChannelStatus.Attributes {
				for key, value := range attr.Values {
					if value.IsInt64 {

						chlType := int(mqmetric.ChannelStatus.Attributes[mqmetric.ATTR_CHL_TYPE].Values[key].ValueInt64)
						chlTypeString := strings.Replace(ibmmq.MQItoString("CHT", chlType), "MQCHT_", "", -1)
						// Not every channel status report has the RQMNAME attribute (eg SVRCONNs)
						rqmName := ""
						if rqmNameAttr, ok := mqmetric.ChannelStatus.Attributes[mqmetric.ATTR_CHL_RQMNAME].Values[key]; ok {
							rqmName = rqmNameAttr.ValueString
						}

						chlName := mqmetric.ChannelStatus.Attributes[mqmetric.ATTR_CHL_NAME].Values[key].ValueString
						connName := mqmetric.ChannelStatus.Attributes[mqmetric.ATTR_CHL_CONNNAME].Values[key].ValueString
						jobName := mqmetric.ChannelStatus.Attributes[mqmetric.ATTR_CHL_JOBNAME].Values[key].ValueString

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
						fields := map[string]interface{}{attr.MetricName: f}

						pt, _ := client.NewPoint(series, tags, fields, t)

						bp.AddPoint(pt)
						log.Debugf("Adding channel point %v", pt)
					}
				}
			}

			series = "queue"
			for _, attr := range mqmetric.QueueStatus.Attributes {
				for key, value := range attr.Values {
					if value.IsInt64 {

						qName := mqmetric.QueueStatus.Attributes[mqmetric.ATTR_Q_NAME].Values[key].ValueString

						tags := map[string]string{
							"qmgr": config.cf.QMgrName,
						}
						tags["queue"] = qName
						tags["platform"] = platformString

						f := mqmetric.QueueNormalise(attr, value.ValueInt64)
						fields := map[string]interface{}{attr.MetricName: f}

						pt, _ := client.NewPoint(series, tags, fields, t)

						bp.AddPoint(pt)
						log.Debugf("Adding queue point %v", pt)
					}
				}
			}

			series = "topic"
			for _, attr := range mqmetric.TopicStatus.Attributes {
				for key, value := range attr.Values {
					if value.IsInt64 {

						topicString := mqmetric.TopicStatus.Attributes[mqmetric.ATTR_TOPIC_STRING].Values[key].ValueString
						topicStatusType := mqmetric.TopicStatus.Attributes[mqmetric.ATTR_TOPIC_STATUS_TYPE].Values[key].ValueString

						tags := map[string]string{
							"qmgr": config.cf.QMgrName,
						}
						tags["topic"] = topicString
						tags["platform"] = platformString
						tags["type"] = topicStatusType

						f := mqmetric.TopicNormalise(attr, value.ValueInt64)
						fields := map[string]interface{}{attr.MetricName: f}

						pt, _ := client.NewPoint(series, tags, fields, t)

						bp.AddPoint(pt)
						log.Debugf("Adding topic point %v", pt)
					}
				}
			}

			series = "qmgr"
			for _, attr := range mqmetric.QueueManagerStatus.Attributes {
				for key, value := range attr.Values {
					if value.IsInt64 {

						qMgrName := mqmetric.QueueManagerStatus.Attributes[mqmetric.ATTR_QMGR_NAME].Values[key].ValueString

						tags := map[string]string{
							"qmgr":     qMgrName,
							"platform": platformString,
						}

						f := mqmetric.QueueManagerNormalise(attr, value.ValueInt64)
						fields := map[string]interface{}{attr.MetricName: f}

						pt, _ := client.NewPoint(series, tags, fields, t)

						bp.AddPoint(pt)
						log.Debugf("Adding qmgr point %v", pt)
					}
				}
			}
		}
		// This is where real errors might occur, including the inability to
		// contact the database server. We will ignore (but log)  these errors
		// up to a threshold, after which it is considered fatal.
		err = c.Write(bp)
		if err != nil {
			log.Error(err)
			errorCount++
			if errorCount >= config.maxErrors {
				log.Fatal("Too many errors communicating with server")
			}
		} else {
			errorCount = 0
		}
	}

	return err

}
