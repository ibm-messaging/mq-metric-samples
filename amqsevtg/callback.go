/*
 * This is an example of a Go program to format IBM MQ Event messages.
 * It is essentially a rewrite of the product's sample program amqsevta.c
 *
 * Functions in this file deal with the asynchronous delivery of messages
 * via Callbacks, and then formatting them for JSON output
 */
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
   See the License for the specific language governing permissions and
   limitations under the license.

	Contributors:
	  Mark Taylor - Initial Contribution
*/

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"

	mq "github.com/ibm-messaging/mq-golang/v5/ibmmq"
)

// This is the Callback function invoked when there are messages on queues or when
// some other special event happens
func cb(hConn *mq.MQQueueManager, hObj *mq.MQObject, md *mq.MQMD, gmo *mq.MQGMO, buffer []byte, cbc *mq.MQCBC, err *mq.MQReturn) {

	switch cbc.CallType {
	case mq.MQCBCT_MSG_REMOVED,
		mq.MQCBCT_MSG_NOT_REMOVED:

		oName := ""
		oType := mq.MQOT_Q
		for i := 0; i < len(openObjects); i++ { // Which topic/queue did message come from
			if openObjects[i].hObj == hObj {
				oName = openObjects[i].objectName
				oType = openObjects[i].objectType
				openObjects[i].active = true
				if openObjects[i].objectType == mq.MQOT_TOPIC {
					// If we've subscribed via a wildcard topic, then the real topic name is useful
					// That is contained in a message property
					if int32(gmo.MsgHandle.GetValue()) != mq.MQHM_UNUSABLE_HMSG {
						oName = getTopic(gmo.MsgHandle)
					}
				}
				break
			}
		}

		/***********************************************************/
		/* Deal with the simplest case of an EPH structure by      */
		/* stepping over it. In theory there could be chained EPH  */
		/* blocks, but that's unlikely.                            */
		/***********************************************************/
		if md.Format == mq.MQFMT_EMBEDDED_PCF {
			buffer = buffer[mq.MQEPH_STRUC_LENGTH_FIXED:]
			cbc.DataLength -= mq.MQEPH_STRUC_LENGTH_FIXED
		}

		// Print out the event. If it is not a PCF-formatted message, then show
		// some of the message data. But do not go overboard with the formatting.
		switch md.Format {

		case mq.MQFMT_EVENT,
			mq.MQFMT_PCF,
			mq.MQFMT_EMBEDDED_PCF,
			mq.MQFMT_ADMIN:
			formatEvent(oName, oType, err.MQRC, md, buffer[0:cbc.DataLength])

		case mq.MQFMT_STRING:
			fmt.Fprintf(statusStream, "String message: <%s>\n", strings.TrimSpace(string(buffer)))

		default:
			// Print the first few bytes of the message
			l := cbc.DataLength
			if l > 50 {
				l = 50
			}
			fmt.Fprintf(statusStream, "Binary message of length %d: %v\n", cbc.DataLength, buffer[0:l])
		}

	case mq.MQCBCT_EVENT_CALL:

		// We might get 2033 for some but not all queues. So we keep track of them
		// and only exit when all the queues have become inactive
		if err.MQRC == mq.MQRC_NO_MSG_AVAILABLE {
			someActive := false
			for i := 0; i < len(openObjects); i++ {
				o := openObjects[i]
				if o.hObj == hObj {
					o.active = false
				} else if o.active {
					someActive = true
				}
			}

			// There still might be multiple callbacks for a given queue after all have given a 2033
			// so it's cleaner to only print this message once.
			// Note that because MQ only uses a single thread for all of its callbacks for a given hConn,
			// serialising the invocations, we don't have to worry about any race conditions with other threads.
			if !someActive {
				if !noMorePrinted {
					fmt.Fprintf(statusStream, "**** All queues have reported no more available messages ****\n")
					noMorePrinted = true
				}
				endProgram = true
			}

		} else if err.MQRC == mq.MQRC_OBJECT_CHANGED ||
			err.MQRC == mq.MQRC_CONNECTION_BROKEN ||
			err.MQRC == mq.MQRC_Q_MGR_STOPPING ||
			err.MQRC == mq.MQRC_Q_MGR_QUIESCING ||
			err.MQRC == mq.MQRC_CONNECTION_QUIESCING ||
			err.MQRC == mq.MQRC_CONNECTION_STOPPING {

			fmt.Fprintf(statusStream, "**** Event Call Reason = %d [%s] Object: %s ****\n",
				err.MQRC, mq.MQItoString("MQRC", int(err.MQRC)), hObj.Name)

			endProgram = true
		}

	default:
		fmt.Fprintf(statusStream, "\n")
		fmt.Fprintf(statusStream, "**** Unexpected CallType = %d\n ****", cbc.CallType)

	}
}

// The formatEvent function does the real work.
// It's where the event message is turned into JSON elements and printed.
func formatEvent(objectName string, objectType int32, callbackReason int32, md *mq.MQMD, buf []byte) {
	var cfh *mq.MQCFH
	var elem *mq.PCFParameter

	parmAvail := true
	bytesRead := 0
	offset := 0
	datalen := len(buf)

	var event Event

	cfh, offset = mq.ReadPCFHeader(buf)
	if cfh == nil || cfh.ParameterCount == 0 {
		return
	}

	// Check the data
	if cfh.Type > mq.MQCFT_STATUS {
		fmt.Fprintf(statusStream, "*** Message is not in event message range. It is of type %d\n", cfh.Type)
		return
	}

	// Verify that it's the right version
	if cfh.Version < mq.MQCFH_VERSION_1 || cfh.Version > mq.MQCFH_CURRENT_VERSION {
		fmt.Fprintf(statusStream, "*** Header is the wrong version, %d\n", cfh.Version)
		return
	}

	event.EventSource.ObjectName = objectName
	event.EventSource.ObjectType = prettyVal(mq.MQItoString("MQOT", int(objectType)))
	event.EventSource.QueueManager = cf.QMgrName

	event.EventType.Name = prettyVal(mq.MQItoString("MQCMD", int(cfh.Command)))
	event.EventType.Value = cfh.Command

	event.EventReason.Name = prettyVal(mq.MQItoString("MQRC", int(cfh.Reason)))
	event.EventReason.Value = cfh.Reason

	// Extract the PutDate/PutTime from the MQMD and format it in 2 ways
	timestamp := md.PutDateTime.Format("2006-01-02T15:04:05.99Z")
	epoch := md.PutDateTime.Unix()

	event.EventCreation.TimeStamp = timestamp
	event.EventCreation.Epoch = epoch

	if cfh.Type != mq.MQCFT_EVENT {
		event.MsgType = new(NV)
		event.MsgType.Name = prettyVal(mq.MQItoString("MQCFT", int(cfh.Type)))
		event.MsgType.Value = cfh.Type
	}

	if callbackReason != 0 {
		event.CallbackReason = new(NV)
		event.CallbackReason.Name = prettyVal(mq.MQItoString("MQRC", int(callbackReason)))
		event.CallbackReason.Value = callbackReason
	}

	// Config events have before/after status indicated
	// by the Control field in the event.
	if cfh.Reason == mq.MQRC_CONFIG_CHANGE_OBJECT {
		state := STATE_BEFORE_CHANGE
		if cfh.Control == mq.MQCFC_LAST {
			state = STATE_AFTER_CHANGE
		}
		event.ObjectState = state
	}

	// The CorrelId is used to tie config events to each other
	// and to command events
	if cfh.Command == mq.MQCMD_CONFIG_EVENT || cfh.Command == mq.MQCMD_COMMAND_EVENT {
		event.CorrelId = hex.EncodeToString(md.CorrelId)
	}

	// This is where we put the rest of the decoded elements
	body := make(map[string]interface{})

	for parmAvail && cfh.CompCode != mq.MQCC_FAILED {
		elem, bytesRead = mq.ReadPCFParameter(buf[offset:])
		offset += bytesRead
		// Have we now reached the end of the message
		if offset >= datalen {
			parmAvail = false
		}

		if elem.Type == mq.MQCFT_GROUP {
			var arr []interface{}
			var ok bool
			cnt := elem.ParameterCount
			newBody := make(map[string]interface{})
			groupName := mq.MQItoString("MQGACF", int(elem.Parameter))
			groupName = prettyName(groupName)

			// These groups can have multiple records of the same type, so we make them into
			// array entries
			if elem.Parameter == mq.MQGACF_ACTIVITY_TRACE ||
				elem.Parameter == mq.MQGACF_Q_ACCOUNTING_DATA ||
				elem.Parameter == mq.MQGACF_Q_STATISTICS_DATA ||
				elem.Parameter == mq.MQGACF_CHL_STATISTICS_DATA {
				arr, ok = body[groupName].([]interface{})
				if !ok {
					arr = make([]interface{}, 0)
				}
				arr = append(arr, newBody)
				body[groupName] = arr
			} else {
				// Other groups only have a single component
				body[groupName] = newBody
			}

			// There are no cases of nested groups in MQ events, so we can just walk
			// through the elements without worrying about recursion
			for i := 0; i < int(cnt); i++ {
				elem := elem.GroupList[i]
				printElem(elem, newBody)
			}
		} else {
			printElem(elem, body)
		}
	}

	event.EventData = body

	// The event message has been formatted into the structure, so we
	// can now print it.
	var data []byte

	if cf.OTelEndpoint == "" {
		if cf.compact {
			data, _ = json.Marshal(event)
		} else {
			data, _ = json.MarshalIndent(event, "", "  ")
		}

		// If printing in the JSON Array style, then we need to start with the array
		// indicator. All except the first message then needs a comma to separate the
		// entry
		if cf.jsonArray {
			if firstEvent {
				fmt.Printf("[\n")
				firstEvent = false
			} else {
				separator = ","
			}
		}

		fmt.Printf("%s%s\n", separator, data)
	} else {
		otelLog(event)
	}
}

// Look at the message properties to find out the real topic name
func getTopic(hMsg mq.MQMessageHandle) string {
	rc := ""
	impo := mq.NewMQIMPO()
	pd := mq.NewMQPD()

	impo.Options = mq.MQIMPO_CONVERT_VALUE | mq.MQIMPO_INQ_FIRST
	_, value, err := hMsg.InqMP(impo, pd, "MQTopicString")
	if err == nil {
		rc = value.(string)
	} else {
		fmt.Fprintf(statusStream, "Cannot read Topic property: %v", err)
	}

	return rc
}
