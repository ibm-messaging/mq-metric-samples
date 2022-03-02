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

import (
	"encoding/json"
	"fmt"
	mq "github.com/ibm-messaging/mq-golang/v5/ibmmq"
	log "github.com/sirupsen/logrus"
	"os"
	"strings"
)

var (
	hConn     mq.MQQueueManager
	hObjReply mq.MQObject
	hObj      mq.MQObject

	objName string
	objType = mq.MQOT_Q
)

func main() {

	initParms()
	err := parseParms()
	if err != nil {
		os.Exit(1)
	}

	for _, arg := range os.Args[1:] {
		o.Info.Parms += arg + " "
	}
	o.Info.Parms = strings.TrimSpace(o.Info.Parms)

	/*
	 * Connect to the qmgr. If that is OK, then open both
	 * the requested object (queue or topic) and then
	 * the replyQ
	 */
	err = connect()
	if err == nil {
		log.Debugf("Successfully connected to " + cf.QMgrName)
	}

	if err == nil {
		err = openReplyQ()
		if err == nil {
			log.Debugf("Successfully opened " + cf.ReplyQ)
		}
	}

	if err == nil {
		objName, err = openOutput()
		if err == nil {
			log.Debugf("Successfully opened " + objName)
		}
	}

	// The msgCount value allows testing of cluster distribution by sending multiple tracers and seeing
	// what the final destination is.
	for i := 0; i < cf.MsgCount; i++ {
		// If it's all OK so far, then we can start to do the tracing. Ignore errors from the
		// doTrace function as we still want to test all the possible cluster destinations
		if err == nil {
			doTrace(i)
		}
		// Always print any collected data
		printSummary()
	}

	// And cleanup
	shutdown()
}

func shutdown() {
	if hObjReply.Name != "" {
		hObjReply.Close(0)
	}
	if hObj.Name != "" {
		hObj.Close(0)
	}

	if hConn.Name != "" {
		if backoutRequired {
			hConn.Back()
		}
		hConn.Disc()
	}
}

// Create and fill in the structures that will be written as JSON.
//
func printSummary() {
	var firstEpoch int64
	var lastEpoch int64

	// The activities might have been reported out of sequence, so they have
	// been stored in a map rather than a simple array/list. We then retrieve them from
	// the map and build a list so they are printed in correct order.
	// The initial "test" message activity is put on the start of the array explicitly,
	// and we then walk through the map from key=1. The "immediateFailure"
	// value means that we shouldn't attempt to build the activity list as we never got that far
	if !o.immediateFailure {
		o.Activities = make([]*Activity, 0)
		o1 := o.FirstActivity
		o.Activities = append(o.Activities, o1)

		if len(o1.Operations) > 0 {
			firstRealOp := o1.Operations[0]
			firstEpoch = firstRealOp.TimeStampEpoch
		}

		for i := 1; i <= highestActivity; i++ {
			if act, ok := activityMap[i]; ok {
				o.Activities = append(o.Activities, act)

				// We need the last time too
				lastOp := act.Operations[len(act.Operations)-1]
				o.Info.LastTime = lastOp.Time
				o.Info.LastDate = lastOp.Date

				o.Info.LastTimeStampEpoch = lastOp.TimeStampEpoch
				lastEpoch = lastOp.TimeStampEpoch

				// Was there a successful completion? Did we
				// traverse a channel?
				for j := 0; j < len(act.Operations); j++ {
					op := act.Operations[j]
					if op != nil {
						if op.operationVal == mq.MQOPER_SEND {
							o.Info.ChannelCount++
						} else if op.operationVal == mq.MQOPER_DISCARD {
							s := op.Details[STR_QMGR]
							if s != nil {
								o.Info.FinalQMgr = s.(string)
								o.Info.Success = STR_SUCCESSFUL
							}
						}
					}
				}
			}
		}
		if o.Info.Success == "" && len(o.Error) == 0 {
			o.Info.Failure = STR_FAILURE
		}
		o.Info.TotalTime = timeDiff(firstEpoch, lastEpoch)
	}
	// And now serialise the whole thing as a pretty-printed JSON structure
	j := make([]byte, 0)
	if cf.OneLine {
		j, _ = json.Marshal(o)

	} else {
		j, _ = json.MarshalIndent(o, "", "  ")
	}
	fmt.Printf("%s\n", string(j))
}

/* Connect to the requested qmgr
 */
func connect() error {
	var err error
	var gocd *mq.MQCD

	log.Debugf("InitConnection: QMgrName %s", cf.QMgrName)

	gocno := mq.NewMQCNO()
	gocsp := mq.NewMQCSP()

	// Explicitly force client mode if requested. Otherwise use the "default"
	// Client mode can be come from a simple boolean, or from having
	// common configurations with the CCDT or ConnName/Channel being set.
	// Anything involving TLS will require the CCDT mode
	if cf.CcdtUrl != "" {
		cf.ClientMode = true
	} else if cf.ConnName != "" || cf.Channel != "" {
		cf.ClientMode = true
		gocd = mq.NewMQCD()
		gocd.ChannelName = cf.Channel
		gocd.ConnectionName = cf.ConnName
	}

	// connection mechanism depending on what is installed or configured.
	if cf.ClientMode {
		gocno.Options = mq.MQCNO_CLIENT_BINDING
		if cf.CcdtUrl != "" {
			gocno.CCDTUrl = cf.CcdtUrl
			log.Debugf("Trying to connect as client using CCDT: %s", gocno.CCDTUrl)
		} else if gocd != nil {
			gocno.ClientConn = gocd
			log.Debugf("Trying to connect as client using ConnName: %s, Channel: %s", gocd.ConnectionName, gocd.ChannelName)
		} else {
			log.Debugf("Trying to connect as client with external configuration")
		}
	}
	gocno.Options |= mq.MQCNO_HANDLE_SHARE_BLOCK

	// The password will have already been read during config parsing
	if cf.Password != "" {
		gocsp.Password = cf.Password
	}
	if cf.UserId != "" {
		gocsp.UserId = cf.UserId
		gocno.SecurityParms = gocsp
	}

	log.Debugf("Connecting to queue manager %s", cf.QMgrName)
	hConn, err = mq.Connx(cf.QMgrName, gocno)
	if err != nil {
		addErr("Cannot connect to queue manager "+cf.QMgrName, err)
		o.immediateFailure = true
	}

	return err
}

// The replyQ for picking up the responses
func openReplyQ() error {
	var err error
	od := mq.NewMQOD()
	od.ObjectName = cf.ReplyQ

	openOptions := mq.MQOO_INPUT_AS_Q_DEF | mq.MQOO_OUTPUT | mq.MQOO_NO_READ_AHEAD | mq.MQOO_BROWSE | mq.MQOO_FAIL_IF_QUIESCING
	hObjReply, err = hConn.Open(od, openOptions)
	if err != nil {
		addErr("Cannot open object "+cf.ReplyQ, err)
		o.immediateFailure = true
	}
	cf.realReplyQ = od.ObjectName
	return err

}

// The real target queue/topic
func openOutput() (string, error) {
	var err error
	od := mq.NewMQOD()
	od.Version = 4 // Needed to make sure we get the resolution of topics
	openOptions := mq.MQOO_OUTPUT
	realName := ""
	if cf.TargetTopic != "" {
		od.ObjectString = cf.TargetTopic
		od.ObjectType = mq.MQOT_TOPIC
		realName = cf.TargetTopic
		openOptions |= mq.MQOO_OUTPUT | mq.MQOO_FAIL_IF_QUIESCING
	} else if cf.TargetQ != "" {
		od.ObjectName = cf.TargetQ
		realName = cf.TargetQ
		if cf.TargetQMgr != "" {
			od.ObjectQMgrName = cf.TargetQMgr
		}
		od.ObjectType = mq.MQOT_Q
		openOptions |= cf.Bind | mq.MQOO_NO_READ_AHEAD | mq.MQOO_FAIL_IF_QUIESCING
	}

	hObj, err = hConn.Open(od, openOptions)
	if err != nil {
		addErr("Cannot open "+objTypeString(objType)+" "+od.ObjectName, err)
		o.immediateFailure = true
	} else {
		// The underlying object type might have changed when there's a QALIAS pointing to a topic
		objType = od.ResolvedType
		if od.ResolvedType == mq.MQOT_TOPIC {
			realName = od.ResObjectString
		} else {
			realName = od.ObjectName
		}
	}

	return realName, err
}
