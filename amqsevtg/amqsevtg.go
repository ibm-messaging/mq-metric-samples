/*
 * This is an example of a Go program to format IBM MQ Event messages.
 * It is essentially a rewrite of the product's sample program amqsevta.c
 *
 * One primary difference from the C program is that only JSON-formatted
 * output is available. Another difference is that the order of elements in
 * the events may vary - not just between the C and Go versions, but
 * within the Go output itself, as the language's "map" construct gives no
 * ordering guarantees.
 *
 * There is also an option to send the event messages directly to an
 * OpenTelemetry backend.
 */
package main

/*
  Copyright (c) IBM Corporation 2025

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
	"fmt"
	"os"
	"strings"
	"time"

	mq "github.com/ibm-messaging/mq-golang/v5/ibmmq"
)

type OpenObject struct {
	hObj       *mq.MQObject
	objectType int32
	objectName string
	active     bool
	hSub       *mq.MQObject
	msgHandle  *mq.MQMessageHandle
}

// Define some structures that will be used for elements of the formatted event
type NV struct {
	Name  string `json:"name"`
	Value int32  `json:"value"`
}
type TS struct {
	TimeStamp string `json:"timeStamp"`
	Epoch     int64  `json:"epoch"`
}
type ES struct {
	ObjectName   string `json:"objectName"`
	ObjectType   string `json:"objectType"`
	QueueManager string `json:"queueManager"`
}

// The main structure
type Event struct {
	EventSource    ES             `json:"eventSource"`
	EventType      NV             `json:"eventType"`
	MsgType        *NV            `json:"msgType,omitempty"`        // This is a pointer so that it can be nil
	CallbackReason *NV            `json:"callbackReason,omitempty"` // This is a pointer so that it can be nil
	EventReason    NV             `json:"eventReason"`
	EventCreation  TS             `json:"eventCreation"`
	CorrelId       string         `json:"correlationID,omitempty"`
	ObjectState    string         `json:"objectState,omitempty"`
	EventData      map[string]any `json:"eventData,omitempty"`
}

const (
	// Put hardcoded string values in here
	STATE_BEFORE_CHANGE = "Before Change"
	STATE_AFTER_CHANGE  = "After Change"
	ATTRS_ALL           = "All"
)

var (
	hConn mq.MQQueueManager

	openObjects     []*OpenObject
	openObjectCount = -1

	platform  int32
	bigBuffer = make([]byte, 0, 1024*1024*4) // 4MB should be large enough to avoid truncation and needing to reallocate

	separator     = "" // Will set to ',' a bit later for array-formatted output
	firstEvent    = true
	usingDefaults = false
	endProgram    = false
	noMorePrinted = false

	// Anything that is not part of the JSON message goes to stderr to make piping easy
	statusStream = os.Stderr

	// These are the standard event queues, used if no
	// specific objects are given on the command line.
	// Some queues may not be available on all versions/platforms,
	// so we ignore errors from those when we're using this list.
	defaultQueues = "SYSTEM.ADMIN.PERFM.EVENT," +
		"SYSTEM.ADMIN.CHANNEL.EVENT," +
		"SYSTEM.ADMIN.QMGR.EVENT," +
		"SYSTEM.ADMIN.LOGGER.EVENT," +
		"SYSTEM.ADMIN.PUBSUB.EVENT," +
		"SYSTEM.ADMIN.CONFIG.EVENT," +
		"SYSTEM.ADMIN.COMMAND.EVENT"
)

func main() {
	os.Exit(mainWithRc())
}

// The real main function is here to set a return code.
func mainWithRc() int {
	var err error

	initParms()
	err = parseParms()
	if err != nil {
		fmt.Fprintf(statusStream, "%v\n", err)
		return (1)
	}

	if cf.OTelEndpoint != "" {
		err = initOTel()
		if err != nil {
			fmt.Fprintf(statusStream, "%v\n", err)
			return (1)
		}

		defer shutdownOTel()
	}

	// After this point, "err" should always be an MQReturn type

	// This is where we connect to the queue manager. It is assumed
	// that the queue manager is either local, or you have set the
	// client connection information externally eg via a CCDT or the
	// MQSERVER environment variable.
	cno := mq.NewMQCNO()
	if cf.ClientMode {
		cno.Options |= mq.MQCNO_CLIENT_BINDING
	}
	if cf.reconnectDisabled {
		cno.Options |= mq.MQCNO_RECONNECT_DISABLED
	} else if cf.reconnectQMgr {
		cno.Options |= mq.MQCNO_RECONNECT_Q_MGR
	} else if cf.reconnectAny {
		cno.Options |= mq.MQCNO_RECONNECT
	}

	if cf.UserId != "" {
		csp := mq.NewMQCSP()
		csp.UserId = cf.UserId
		csp.Password = cf.password
		cno.SecurityParms = csp
	}

	hConn, err = mq.Connx(cf.QMgrName, cno)

	if err != nil {
		fmt.Fprintln(statusStream, err)
	} else {
		defer disc(hConn)
	}

	// Work out what platform we are on
	if err == nil {
		var hObj mq.MQObject
		mqod := mq.NewMQOD()
		Options := mq.MQOO_INQUIRE | mq.MQOO_FAIL_IF_QUIESCING
		mqod.ObjectType = mq.MQOT_Q_MGR

		//   Open the queue manager for an inquire operation
		hObj, err = hConn.Open(mqod, Options)
		if err != nil {
			fmt.Fprintln(statusStream, err)

		} else {
			var values map[int32]any
			selectors := []int32{mq.MQIA_PLATFORM}
			values, err = hObj.Inq(selectors)
			if err == nil {
				platform = values[mq.MQIA_PLATFORM].(int32)
			}
			hObj.Close(0)
		}
	}

	// Open of the queues/topics
	if err == nil {

		if cf.QName == "" && cf.TopicName == "" {
			cf.QName = defaultQueues
			usingDefaults = true
		}

		if cf.QName != "" {
			objectList := strings.Split(cf.QName, ",")
			for i := 0; i < len(objectList); i++ {
				// We know that this queue will never exist on z/OS
				if platform == mq.MQPL_ZOS && objectList[i] == "SYSTEM.ADMIN.LOGGER.EVENT" {
					continue
				} else {
					newOpenObject(objectList[i], mq.MQOT_Q)
				}
			}
		}

		if cf.TopicName != "" {
			objectList := strings.Split(cf.TopicName, ",")
			for i := 0; i < len(objectList); i++ {
				newOpenObject(objectList[i], mq.MQOT_TOPIC)
			}
		}
	}

	for i := 0; i < len(openObjects) && err == nil; i++ {
		var mh mq.MQMessageHandle
		o := openObjects[i]
		cmho := mq.NewMQCMHO()
		mh, err = hConn.CrtMH(cmho)
		if err != nil {
			fmt.Fprintln(statusStream, err)
		} else {
			o.msgHandle = &mh
			defer dltMh(mh)
		}
	}

	for i := 0; i < len(openObjects) && err == nil; i++ {
		var hObj mq.MQObject
		o := openObjects[i]

		if o.objectType == mq.MQOT_Q {
			mqod := mq.NewMQOD()
			openOptions := mq.MQOO_INPUT_SHARED
			if cf.Browse {
				openOptions = mq.MQOO_BROWSE
			}

			mqod.ObjectType = mq.MQOT_Q
			mqod.ObjectName = o.objectName

			hObj, err = hConn.Open(mqod, openOptions)
			if err != nil {
				mqret := err.(*mq.MQReturn)
				if mqret.MQRC == mq.MQRC_UNKNOWN_OBJECT_NAME && usingDefaults {
					err = nil // Ignore this one if we're using the default objects
				} else {
					fmt.Fprintln(statusStream, err)
				}
			} else {
				o.hObj = &hObj
				o.active = true
				defer close(hObj)
			}
		} else {
			mqsd := mq.NewMQSD()
			mqsd.Options = mq.MQSO_CREATE |
				mq.MQSO_NON_DURABLE |
				mq.MQSO_FAIL_IF_QUIESCING |
				mq.MQSO_MANAGED

			mqsd.ObjectString = o.objectName

			hObj, err = hConn.Sub(mqsd, o.hObj)
			if err != nil {
				fmt.Fprintln(statusStream, err)
			} else {
				o.hSub = &hObj
				o.active = true
				defer close(hObj)
			}
		}
	}

	// Register a callback function (the same one) for all of the
	// explicit queues and subscription-managed queues
	for i := 0; i < len(openObjects) && err == nil; i++ {
		o := openObjects[i]

		if !o.active {
			continue
		}
		cbd := mq.NewMQCBD()
		gmo := mq.NewMQGMO()
		md := mq.NewMQMD()
		cbd.CallbackFunction = cb
		cbd.Options |= mq.MQCBDO_FAIL_IF_QUIESCING

		gmo.Options = mq.MQGMO_NO_SYNCPOINT | mq.MQGMO_CONVERT
		gmo.Options |= mq.MQGMO_PROPERTIES_IN_HANDLE
		gmo.Options |= mq.MQGMO_FAIL_IF_QUIESCING

		gmo.MsgHandle = *o.msgHandle
		gmo.MatchOptions = mq.MQMO_NONE

		gmo.WaitInterval = mq.MQWI_UNLIMITED
		if cf.waitInterval != mq.MQWI_UNLIMITED {
			gmo.WaitInterval = cf.waitInterval * 1000
			gmo.Options |= mq.MQGMO_WAIT
		}

		if cf.Browse {
			gmo.Options |= mq.MQGMO_BROWSE_NEXT
		}

		err = o.hObj.CB(mq.MQOP_REGISTER, cbd, md, gmo)
		if err == nil {
			defer dereg(*o.hObj, cbd, md, gmo)
		}
	}

	if err == nil {
		// Then we are ready to enable the callback function. Any messages
		// on the queue will be sent to the callback
		ctlo := mq.NewMQCTLO()
		err = hConn.Ctl(mq.MQOP_START, ctlo)
		if err == nil {
			// Use defer to disable the message consumer when we are ready to exit.
			// Otherwise the shutdown will give MQRC_HCONN_ASYNC_ACTIVE error
			defer stopCB(hConn)
		}
	}

	// Keep running until either ENTER is pressed or there's an
	// error like running out of messages or qmgr ending
	if err == nil {
		fmt.Fprintf(statusStream, "Press ENTER to end\n")
		go func() {
			fmt.Scanln()
			fmt.Fprintf(statusStream, "Ending...\n")
			endProgram = true
		}()
	}

	dur, _ := time.ParseDuration("500ms")
	for err == nil && !endProgram {
		time.Sleep(dur)
	}

	// Return code is extracted from any failing MQI call.
	// Deferred disconnect/closes will happen after the return
	mqret := 0
	if err != nil {
		mqerr := err.(*mq.MQReturn)

		// One very specific error case that we might be able to diagnose
		if mqerr.MQRC == mq.MQRC_ENVIRONMENT_ERROR && cf.ClientMode {
			fmt.Fprintf(statusStream, "Using MQCB requires non-zero SHARECNV on the SVRCONN configuration.\n")
		}

		mqret = int(mqerr.MQCC)
	}

	// The array closure needs to be printed to stdout, provided we've printed
	// at least one event already. Print this even if there's been an error to get the
	// best possible ending to the output.
	if cf.jsonArray && !firstEvent {
		fmt.Printf("]\n")
	}

	return mqret
}

// Disconnect from the queue manager
func disc(hConn mq.MQQueueManager) error {
	err := hConn.Disc()
	if err == nil {
		// Do nothing
	} else {
		// Print the error, even though there's not much we can do about it
		fmt.Fprintln(statusStream, err)
	}
	return err
}

// Close the queue if it was opened
func close(object mq.MQObject) error {
	err := object.Close(0)
	if err == nil {
		// Do nothing
	} else {
		// Print the error, even though there's not much we can do about it
		fmt.Fprintln(statusStream, err)
	}
	return err
}

func stopCB(hConn mq.MQQueueManager) {
	ctlo := mq.NewMQCTLO()
	hConn.Ctl(mq.MQOP_STOP, ctlo)
}

// Deallocate the message handle
func dltMh(mh mq.MQMessageHandle) {
	dmho := mq.NewMQDMHO()
	mh.DltMH(dmho)
	return
}

// Deregister the callback function - have to do this before the message handle can be
// successfully deleted
func dereg(qObject mq.MQObject, cbd *mq.MQCBD, getmqmd *mq.MQMD, gmo *mq.MQGMO) error {
	err := qObject.CB(mq.MQOP_DEREGISTER, cbd, getmqmd, gmo)
	return err
}

func newOpenObject(name string, objectType int32) *OpenObject {
	o := new(OpenObject)
	o.objectType = objectType
	o.objectName = name
	o.hObj = new(mq.MQObject)
	o.hSub = new(mq.MQObject)
	o.active = false
	openObjects = append(openObjects, o)
	return o
}
