package main

/*
  Copyright (c) IBM Corporation 2022

  Licensed under the Apache License, Version 2.0 (the "License");
  you may not use this file except in compliance with the License.
  You may obtain a copy of the License at

  http://www.apache.org/licenses/LICENSE-2.0

  Unless required by applicable law or agreed to in writing, software
  distributed under the License is distributed on an "AS IS" BASIS,
  WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
  See the License for the specific language governing permissions and
  limitations under the License.

   Contributors:
     Mark Taylor - Initial Contribution
*/

/*
 * Formatting routines, and constant values that define the
 * JSON output fields.
 */

import (
	mq "github.com/ibm-messaging/mq-golang/v5/ibmmq"
	"math"
	"strings"
	"time"
)

const (
	STR_DSPMQRTJ_NAME = "dspmqrtj"
	STR_DSPMQRTJ_DESC = "Display Route Application (GO/JSON Version)"
	STR_SUCCESSFUL    = "Message successfully reached a destination"
	STR_FAILURE       = "Message not reported at a final destination"

	STR_OPERATION = "operation"

	STR_DESCRIPTION    = "description"
	STR_DETAILED_ERROR = "detailedError"
	STR_MQCC           = "mqcc"
	STR_MQRC           = "mqrc"

	STR_PUT_OBJECT = "objectName"
	STR_PUT_TYPE   = "objectType"

	STR_LOCAL_QMGR   = "localQueueManager"
	STR_LOCAL_Q      = "resolvedQueue"
	STR_RESOLVED_Q   = "resolvedQueue"
	STR_TOPIC_STRING = "topicString"

	STR_REMOTE_Q    = "remoteQueue"
	STR_REMOTE_QMGR = "remoteQueueManager"
	STR_QMGR        = "queueManager"
	STR_QUEUE       = "queue"
	STR_XMIT_Q      = "transmissionQueue"

	STR_CHANNEL_NAME = "channelName"
	STR_CHANNEL_TYPE = "channelType"

	STR_FEEDBACK             = "feedback"
	STR_ROUTE_DELIVERY       = "routeDelivery"
	STR_ROUTE_ACCUMULATION   = "routeAccumulation"
	STR_ROUTE_FORWARDING     = "routeForwarding"
	STR_ROUTE_DETAIL         = "routeDetail"
	STR_ROUTE_DECODE_UNKNOWN = "unknown"
	STR_MAX_ACTIVITIES       = "maxActivities"

	STR_RECORDED   = "recordedActivities"
	STR_UNRECORDED = "unrecordedActivites"
	STR_DISCON     = "discontinuityCount"

	STR_TIMESTAMP = "timestamp" // only used as a pseudo-key so we can add date + time

	mqDateTimeFormat = "20060102150405.00"
	mqDateFormat     = "20060102"
	mqTimeFormat     = "150405.00"
)

// This saves every convertor needing to cast the value between int/int32
func mqiToString(s string, v int32) string {
	return mq.MQItoString(s, int(v))
}

func mqiToStringP(s string, v int32) string {
	return prettify(mq.MQItoString(s, int(v)))
}

// Since we're only dealing with two possible object types, it's easier
// to get a nicely-formatted version with a dedicated function
func objTypeString(v int32) string {
	s := "queue"
	if v == mq.MQOT_TOPIC {
		s = "topic"
	}
	return s
}

// Some variation on how to format MQ constants. The default, because it's more likely to be
// used, turned MQXX_ABC_DEF into AbcDef. The alternative will return abcDef
func prettify(s string) string {
	return prettifyJson(s, false)
}
func prettifyJson(s string, json bool) string {
	toks := strings.Split(s, "_")
	if len(toks) > 1 {
		s = ""
		l := len(toks)
		for i := 1; i < l; i++ {
			tok := toks[i]
			if len(toks[i]) > 1 {
				s = s + strings.ToUpper(tok[0:1]) + strings.ToLower(tok[1:])
			} else {
				s = s + tok
			}
		}
		if json {
			s = strings.ToLower(s[0:1]) + s[1:]
		}
	} else if json {
		s = strings.ToLower(s)
	}
	return s
}

func decodeRouteOptions(attr int32, v int32) (string, string) {
	key := STR_ROUTE_DECODE_UNKNOWN
	val := "Unknown"
	switch attr {
	case mq.MQIACF_ROUTE_ACCUMULATION:
		key = STR_ROUTE_ACCUMULATION
		switch v {
		case mq.MQROUTE_ACCUMULATE_NONE:
			val = "None"
		case mq.MQROUTE_ACCUMULATE_IN_MSG:
			val = "InMessage"
		case mq.MQROUTE_ACCUMULATE_AND_REPLY:
			val = "Reply"
		default:
			// Do nothing
		}
	case mq.MQIACF_ROUTE_DELIVERY:
		key = STR_ROUTE_DELIVERY
		if (v & mq.MQROUTE_DELIVER_YES) == mq.MQROUTE_DELIVER_YES {
			val = "Yes"
		} else if (v & mq.MQROUTE_DELIVER_NO) == mq.MQROUTE_DELIVER_NO {
			val = "No"
		}
	case mq.MQIACF_ROUTE_FORWARDING:
		key = STR_ROUTE_FORWARDING
		if (v & mq.MQROUTE_FORWARD_ALL) == mq.MQROUTE_FORWARD_ALL {
			val = "All"
		} else if (v & mq.MQROUTE_FORWARD_IF_SUPPORTED) == mq.MQROUTE_FORWARD_IF_SUPPORTED {
			val = "IfSupported"
		}
	case mq.MQIACF_ROUTE_DETAIL:
		key = STR_ROUTE_DETAIL
		switch v {
		case mq.MQROUTE_DETAIL_LOW:
			val = "Low"
		case mq.MQROUTE_DETAIL_MEDIUM:
			val = "Medium"
		case mq.MQROUTE_DETAIL_HIGH:
			val = "High"
		}
	default:
		// Do nothing
	}
	return key, val
}

// Input strings are MQ's date/time separate strings which we convert in various ways.
// MQ strings are YYYYMMDD HHMMSShh. We return the time and date in a more readable form, and
// an int representing the epoch in milliseconds.
func timeStamp(d string, t string) (string, string, int64) {
	epoch := int64(0)

	// Separate out the hundredths with the '.'
	parsedTime, err := time.Parse(mqDateTimeFormat, d+t[0:6]+"."+t[6:8])
	if err == nil {
		epoch = parsedTime.UnixNano() / (1000 * 1000) // convert to milliseconds
	} else {
		//fmt.Printf("Timestamp error parsing: %s %s %v\n", d, t, err)
	}

	dateString := d[0:4] + "-" + d[4:6] + "-" + d[6:8]
	timeString := t[0:2] + ":" + t[2:4] + ":" + t[4:6] + "." + t[6:8]
	return dateString, timeString, epoch
}

// Inputs are epoch times including milliseconds
func timeDiff(t1 int64, t2 int64) int64 {
	return int64(math.Abs(float64(t2 - t1)))
}
