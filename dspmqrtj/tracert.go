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
	"fmt"
	mq "github.com/ibm-messaging/mq-golang/v5/ibmmq"
	log "github.com/sirupsen/logrus"
	"math"
	"strings"
	"time"
)

var (
	backoutRequired = false
	seenDiscard     = false
	depth           = 0
	recorded        int32
	unrecorded      int32
	discon          int32
	operationDate   = ""
	operationTime   = ""
	act             *Activity
	op              *Operation

	errString = "An error occurred" // Something very generic as default text

)

/*
 * This is the core of the traceroute program. It initiates the tracing process to
 * the designated queue or topic.
 *
 * The "trick" to doing this is that we put a messsage to the same queue twice but using
 * different PMO options to resolve to either the local or a potentially remote queue/qmgr.
 * The first attempt gets backed out. An initial Activity structure is built to represent the
 * local PUT as the qmgr doesn't generate a response for that step.
 *
 * We cannot do a trace to a simple local queue. If the resolved qmgr is the same as the
 * current qmgr, then there's no remote system involved so we get out fast.
 *
 * For normal multi-hop processing, the 2nd MQPUT gets done with all the pcf attributes needed
 * to ask for tracing. We don't give all of the flexibility that the dspmqrte command supports, but
 * there's enough for the standard use case.
 *
 * Then the replies are read (possibly out of sequence) and parsed to build Activity and Operation
 * structures. Each Activity corresponds to a single program (eg the MCA) and it will have one or more
 * Operations (eg GET, SEND).
 *
 * Fields we care about are extracted from each PCF message. We don't bother formatting the MQMD structures
 * as they aren't very helpful with these tracer messages.
 */

func doTrace(cnt int) error {
	var firstOp *Operation
	var err error

	initActivity()
	seenDiscard = false

	// The test message is only put on the first time through the program as it seems to
	// affect cluster workload balancing
	if cnt == 0 {
		firstOp, err = putTestMessage()
	} else {
		firstOp = o.FirstActivity.Operations[0]
	}

	if err == nil {
		err = putMessage(firstOp)
		if err == nil {
			log.Debug("About to try to get message")

			for err == nil {
				err = getMessage()
				if err != nil {
					if mqe, ok := err.(*mq.MQReturn); ok {
						// This error is "expected" so does not get explicitly reported
						// but it's still a cause to exit the loop
						if mqe.MQRC == mq.MQRC_NO_MSG_AVAILABLE {
							err = nil
							break
						}
					}
				} else if (len(activityMap) >= cf.MaxActivities) || (seenDiscard && len(activityMap) >= highestActivity) {
					log.Debugf("Ending because of received count. len(map):%d maxAct:%d seenDiscard:%t highest:%d", len(activityMap), cf.MaxActivities, seenDiscard, highestActivity)
					break
				}
				log.Debugf("Continuing with receiver. len(map):%d maxAct:%d seenDiscard:%t highest:%d", len(activityMap), cf.MaxActivities, seenDiscard, highestActivity)

			}
		}
	}
	if err != nil {
		addErr(errString, err)
	}

	return err
}

/*
 * Put an initial message to make sure it's worth continuing.
 * Back it out so it doesn't go anywhere
 */
func putTestMessage() (*Operation, error) {
	pmo := mq.NewMQPMO()
	md := mq.NewMQMD()

	backoutRequired = true

	pmo.Options = mq.MQPMO_SYNCPOINT | mq.MQPMO_RESOLVE_LOCAL_Q | mq.MQPMO_NEW_MSG_ID | mq.MQPMO_NEW_CORREL_ID
	err := hObj.Put(md, pmo, nil)
	if err != nil {
		errString = "Cannot put to target object " + hObj.Name
		o.immediateFailure = true
		return nil, err
	}

	hConn.Back()
	backoutRequired = false

	resolvedQName := pmo.ResolvedQName
	resolvedQMgrName := pmo.ResolvedQMgrName
	//log.Debugf("PutTestMessage: PMO is %+v", pmo)
	log.Debugf("PutTestMessage: Resolved Q/QMgr is %s @ %s", resolvedQName, resolvedQMgrName)

	// We generate the first report and call it activity 0.
	act := newActivity()
	act.ApplName = STR_DSPMQRTJ_NAME
	act.ApplType = mqiToStringP("AT", mq.MQAT_DEFAULT)
	act.Description = STR_DSPMQRTJ_DESC
	op := newOperation(act)
	op.OperationString = mqiToStringP("OPER", mq.MQOPER_PUT)
	op.operationVal = mq.MQOPER_PUT

	op.Details[STR_PUT_TYPE] = objTypeString(objType)
	if objType == mq.MQOT_Q {
		op.Details[STR_PUT_OBJECT] = objName
	} else {
		op.Details[STR_TOPIC_STRING] = objName
	}
	op.Details[STR_LOCAL_QMGR] = cf.QMgrName
	if resolvedQName != "" {
		op.Details[STR_LOCAL_Q] = resolvedQName
	}

	addActivity(act, 0)
	return op, err
}

/*
 * And this is the real tracer. Construct the PCF message with the fields that
 * ask each qmgr to respond with the activity data, and parse all the responses until
 * we're complete or run out with an error.
 */
func putMessage(firstOp *Operation) error {
	pmo := mq.NewMQPMO()
	//gmo := mq.NewMQGMO()
	md := mq.NewMQMD()
	var buf []byte

	md.ReplyToQ = cf.realReplyQ
	md.ReplyToQMgr = cf.QMgrName

	md.MsgType = mq.MQMT_REQUEST
	md.CodedCharSetId = mq.MQCCSI_DEFAULT
	md.Persistence = mq.MQPER_NOT_PERSISTENT
	md.Expiry = int32(cf.Expiry.Seconds()) * 10
	md.Report = mq.MQRO_ACTIVITY | mq.MQRO_PASS_DISCARD_AND_EXPIRY | mq.MQRO_DISCARD_MSG
	md.Format = mq.MQFMT_ADMIN

	pmo.Options = mq.MQPMO_SYNCPOINT
	pmo.Options |= mq.MQPMO_NEW_MSG_ID
	pmo.Options |= mq.MQPMO_NEW_CORREL_ID
	pmo.Options |= mq.MQPMO_FAIL_IF_QUIESCING

	cfh := mq.NewMQCFH()

	cfh.Version = mq.MQCFH_VERSION_3
	cfh.Type = mq.MQCFT_TRACE_ROUTE
	cfh.Command = mq.MQCMD_TRACE_ROUTE
	cfh.ParameterCount = 0

	// The first element is a PCF Group. Don't add it to the output
	// buffer to begin with. We do that when we know how many elements
	// are in the group
	grpparm := new(mq.PCFParameter)
	grpparm.Type = mq.MQCFT_GROUP
	grpparm.Parameter = mq.MQGACF_TRACE_ROUTE
	cfh.ParameterCount++

	// Then we add the individual pieces to the group
	pcfparm := new(mq.PCFParameter)
	pcfparm.Type = mq.MQCFT_INTEGER
	pcfparm.Parameter = mq.MQIACF_ROUTE_DETAIL
	pcfparm.Int64Value = []int64{int64(mq.MQROUTE_DETAIL_MEDIUM)}
	grpparm.GroupList = append(grpparm.GroupList, pcfparm)

	pcfparm = new(mq.PCFParameter)
	pcfparm.Type = mq.MQCFT_INTEGER
	pcfparm.Parameter = mq.MQIACF_RECORDED_ACTIVITIES
	pcfparm.Int64Value = []int64{0}
	grpparm.GroupList = append(grpparm.GroupList, pcfparm)

	pcfparm = new(mq.PCFParameter)
	pcfparm.Type = mq.MQCFT_INTEGER
	pcfparm.Parameter = mq.MQIACF_UNRECORDED_ACTIVITIES
	pcfparm.Int64Value = []int64{0}
	grpparm.GroupList = append(grpparm.GroupList, pcfparm)

	pcfparm = new(mq.PCFParameter)
	pcfparm.Type = mq.MQCFT_INTEGER
	pcfparm.Parameter = mq.MQIACF_DISCONTINUITY_COUNT
	pcfparm.Int64Value = []int64{0}
	grpparm.GroupList = append(grpparm.GroupList, pcfparm)

	// Ensure we don't get an infinite loop and OOM errors - if no specific max count
	// has been set, then 1000 is high but not stupid. We're not going to use UNLIMITED as the
	// value just in case
	pcfparm = new(mq.PCFParameter)
	pcfparm.Type = mq.MQCFT_INTEGER
	pcfparm.Parameter = mq.MQIACF_MAX_ACTIVITIES
	if cf.MaxActivities == 0 {
		pcfparm.Int64Value = []int64{1000}
	} else {
		pcfparm.Int64Value = []int64{int64(cf.MaxActivities)}
	}
	grpparm.GroupList = append(grpparm.GroupList, pcfparm)

	pcfparm = new(mq.PCFParameter)
	pcfparm.Type = mq.MQCFT_INTEGER
	pcfparm.Parameter = mq.MQIACF_ROUTE_ACCUMULATION
	pcfparm.Int64Value = []int64{int64(mq.MQROUTE_ACCUMULATE_NONE)}
	grpparm.GroupList = append(grpparm.GroupList, pcfparm)

	pcfparm = new(mq.PCFParameter)
	pcfparm.Type = mq.MQCFT_INTEGER
	pcfparm.Parameter = mq.MQIACF_ROUTE_FORWARDING
	pcfparm.Int64Value = []int64{int64(mq.MQROUTE_FORWARD_ALL)}
	grpparm.GroupList = append(grpparm.GroupList, pcfparm)

	pcfparm = new(mq.PCFParameter)
	pcfparm.Type = mq.MQCFT_INTEGER
	pcfparm.Parameter = mq.MQIACF_ROUTE_DELIVERY
	pcfparm.Int64Value = []int64{int64(mq.MQROUTE_DELIVER_NO)}
	grpparm.GroupList = append(grpparm.GroupList, pcfparm)

	// Now build the entire buffer of CFH + GROUP + Elements
	buf = append(cfh.Bytes(), grpparm.Bytes()...)

	// Add any configured additional message body, to help with 
	// checking things like MaxMsgLength values on queues and channels
	msgLength := cf.MsgLength
	if msgLength > 0 {
		msgLength = msgLength * 1024 // Convert from KB
		body := make([]byte, msgLength)
		for i := 0; i < msgLength; i++ {
			body[i] = byte(i % 256)
		}
		buf = append(buf, body...)
	}

	//log.Debugf("using msg buffer of len %d", len(buf))
	//logDebugMulti(formatBuf(buf))

	pmo.Options = mq.MQPMO_SYNCPOINT
	err := hObj.Put(md, pmo, buf)
	if err != nil {
		errString = "Cannot put to target object " + hObj.Name
	}

	// Fill in the other pieces of the initial activity/operation that we need to know
	if err == nil {

		// Add the current timestamp for the PUT to the operation
		now := time.Now()
		d := now.Format("20060102")
		t := strings.Replace(now.Format("150405.00"), ".", "", -1)
		firstOp.Date, firstOp.Time, firstOp.TimeStampEpoch = timeStamp(d, t)

		if firstOp.Details[STR_LOCAL_QMGR] == pmo.ResolvedQMgrName {
			firstOp.OperationString = mqiToStringP("OPER", mq.MQOPER_DISCARD)
			firstOp.operationVal = mq.MQOPER_DISCARD
			err = fmt.Errorf("Cannot trace a local queue %s", cf.TargetQ)
			hConn.Back()
		} else {
			if pmo.ResolvedQName != "" {
				firstOp.Details[STR_REMOTE_Q] = pmo.ResolvedQName
			}
			if pmo.ResolvedQMgrName != "" {
				firstOp.Details[STR_REMOTE_QMGR] = pmo.ResolvedQMgrName
			}
			hConn.Cmit()

		}
		backoutRequired = false

	}

	if err == nil {
		log.Debugf("PutMessage: Successfully sent %d bytes", len(buf))
	} else {
		log.Debugf("PutMessage: Error string is %s", errString)
		addErr(errString, err)
	}
	return err
}

// Get a reply message and call the parser
func getMessage() error {
	var err error
	md := mq.NewMQMD()
	gmo := mq.NewMQGMO()
	gmo.Options = mq.MQGMO_NO_SYNCPOINT
	gmo.Options |= mq.MQGMO_WAIT | mq.MQGMO_CONVERT
	gmo.WaitInterval = int32(cf.MaxWait.Seconds()) * 1000

	buf := make([]byte, 0, 1024*100)
	datalen := 0
	buf, datalen, err = hObjReply.GetSlice(md, gmo, buf)
	log.Debugf("MQGET returned buffer of len %d. Err = %v\n", datalen, err)
	if err == nil {
		//log.Debugf("received msg buffer of len %d", len(buf))
		//logDebugMulti(formatBuf(buf))
		parseMsg(md, buf)
	}
	return err
}

// A single place to optionally add some debug printing
func r(b []byte) (*mq.PCFParameter, int) {
	p, r := mq.ReadPCFParameter(b)
	//log.Debugf("PCFParm (0) %+v", p)
	return p, r
}

/*
 * A response message should be either format MQADMIN or EmbeddedPCF (MQHEPCF)
 * Then there will be a CFH followed by a single PCF parameter. But that is a group
 * containing multiple elements which may in turn be nested groups. The code can handle a bit
 * more sophistication than that, as some other variations of the traceroute/activity reporting can
 * be more complex. But we're not asking for that with the simple trace request we made above
 *
 * A number of variables are global (defined at the top of this file) to simplify the recursion in the
 * parser that deals with nested groups.
 */
func parseMsg(md *mq.MQMD, buf []byte) error {
	var err error
	var cfh *mq.MQCFH
	var eph *mq.MQEPH
	depth = 0
	inEph := false

	if md.Format != mq.MQFMT_ADMIN && md.Format != mq.MQFMT_EMBEDDED_PCF {
		err = fmt.Errorf("Unexpected message format %s returned", md.Format)
		return err
	}

	if md.Format == mq.MQFMT_EMBEDDED_PCF {
		inEph = true
	}

	offset := 0
	bytesRead := 0

	for inEph {

		if inEph {
			inEph = true

			eph, bytesRead = mq.ReadPCFEmbeddedHeader(buf)
			//log.Debugf("EPH: br=%d %+v", bytesRead, eph)
			offset += bytesRead
			cfh, bytesRead = mq.ReadPCFHeader(buf[offset:])
			offset += bytesRead

			if eph.Format == mq.MQFMT_EMBEDDED_PCF {
				inEph = true
			} else {
				inEph = false
			}
		} else {
			cfh, bytesRead = mq.ReadPCFHeader(buf[offset:])
			offset += bytesRead
			eph = nil
		}

		cnt := cfh.ParameterCount
		//log.Debugf("CFH: %+v", cfh)

		pcfArray := make([]*mq.PCFParameter, cnt)
		index := 0

		for offset < len(buf) {
			pcfArray[index], bytesRead = mq.ReadPCFParameter(buf[offset:])
			offset += bytesRead
			logDebugMulti(pcfDebug(pcfArray[index]))
			index += 1
		}

		for i := 0; i < len(pcfArray); i++ {
			operationDate = ""
			operationTime = ""
			parseParm(pcfArray[i])

			if recorded != -1 && unrecorded != -1 {
				addActivity(act, int(recorded))
				recorded = -1
				unrecorded = -1
			}

			if discon != -1 {
				discon = -1 // TODO:  something here
			}

			if act != nil {
				log.Debugf("Current Activity is %+v", act)
				for i := 0; i < len(act.Operations); i++ {
					log.Debugf("   Operation is %+v", act.Operations[i])
				}
			}
		}
	}
	return err
}

/*
 * Work on the individual parameter, possibly recursively to find the fields
 * of interest
 */
func parseParm(p *mq.PCFParameter) {
	pa := int(p.Parameter)
	parm := ""

	if p.Parameter == mq.MQGACF_ACTIVITY {
		act = newActivity()
		op = nil
		recorded = -1
		unrecorded = -1
	} else if p.Parameter == mq.MQGACF_OPERATION {
		op = newOperation(act)
	}

	switch p.Type {
	case mq.MQCFT_INTEGER:
		parm = mq.MQItoString("IA", pa)
		v := int32(p.Int64Value[0])

		switch p.Parameter {
		case mq.MQIACF_FEEDBACK:
			fb := mqiToString("MQFB", v)
			if fb == "" {
				fb = mqiToString("MQRC", v)
			}
			op.Details[STR_FEEDBACK] = prettify(fb)
		case mq.MQIA_APPL_TYPE:
			act.ApplType = mqiToStringP("MQAT", v)
		case mq.MQIACF_RECORDED_ACTIVITIES:
			recorded = v
			act.TraceRoute[STR_RECORDED] = int(v)
		case mq.MQIACF_UNRECORDED_ACTIVITIES:
			unrecorded = v
			act.TraceRoute[STR_UNRECORDED] = int(v)
		case mq.MQIACF_DISCONTINUITY_COUNT:
			discon = v
			act.TraceRoute[STR_DISCON] = int(v)
		case mq.MQIACF_OPERATION_TYPE:
			op.OperationString = mqiToStringP("OPER", v)
			op.operationVal = v
			if v == mq.MQOPER_DISCARD {
				seenDiscard = true
			}
		case mq.MQIACH_CHANNEL_TYPE:
			op.Details[STR_CHANNEL_TYPE] = mqiToStringP("CHT", v)
		case mq.MQIACF_ROUTE_ACCUMULATION,
			mq.MQIACF_ROUTE_DELIVERY,
			mq.MQIACF_ROUTE_FORWARDING,
			mq.MQIACF_ROUTE_DETAIL:
			key, val := decodeRouteOptions(p.Parameter, v)
			act.TraceRoute[key] = val
		case mq.MQIACF_MAX_ACTIVITIES:
			act.TraceRoute[STR_MAX_ACTIVITIES] = v
		default:
			log.Debugf("Not adding %s [%d] to operation", parm, p.Parameter)
			// do nothing;
		}

	case mq.MQCFT_STRING:
		parm = mq.MQItoString("IA", pa)
		key := ""
		s := p.String[0]
		switch p.Parameter {
		case mq.MQCACF_OPERATION_TIME:
			operationTime = s
			key = STR_TIMESTAMP
		case mq.MQCACF_OPERATION_DATE:
			operationDate = s
			key = STR_TIMESTAMP
		case mq.MQCA_Q_NAME:
			key = STR_QUEUE
		case mq.MQCACF_RESOLVED_Q_NAME:
			if s != "" {
				key = STR_RESOLVED_Q
			} else {
				key = ""
			}
		case mq.MQCA_Q_MGR_NAME:
			key = STR_QMGR
		case mq.MQCACH_CHANNEL_NAME:
			key = STR_CHANNEL_NAME
		case mq.MQCA_REMOTE_Q_MGR_NAME:
			key = STR_REMOTE_QMGR
		case mq.MQCA_REMOTE_Q_NAME:
			key = STR_REMOTE_Q
		case mq.MQCACF_APPL_NAME:
			act.ApplName = s
		case mq.MQCACF_ACTIVITY_DESC:
			act.Description = s
		case mq.MQCACH_XMIT_Q_NAME:
			key = STR_XMIT_Q
		case mq.MQCA_TOPIC_STRING:
			if s != "" {
				key = STR_TOPIC_STRING
			} else {
				key = ""
			}
		default:
			log.Debugf("Not adding %s [%d] to operation", parm, p.Parameter)
			// Do nothing
		}
		if op != nil && key != "" {
			// We need to combine two separate fields from the group in order
			// to create the timestamp. It is safe to use a gloval
			if key == STR_TIMESTAMP {
				if operationDate != "" && operationTime != "" {
					d, t, e := timeStamp(operationDate, operationTime)
					operationDate = ""
					operationTime = ""
					op.Date = d
					op.Time = t
					op.TimeStampEpoch = e
				}
			} else {
				//log.Debugf("Setting %s to %s", key, s)
				if key != "" {
					op.Details[key] = s
				}
			}
		}

	case mq.MQCFT_GROUP:
		switch int32(pa) {
		// Skip dumping large groups that I don't care about
		case mq.MQGACF_EMBEDDED_MQMD, mq.MQGACF_MQMD, mq.MQGACF_MESSAGE:
			// do nothing
		default:
			for i := 0; i < int(p.ParameterCount); i++ {
				parseParm(p.GroupList[i])
			}
		}
	default:
		log.Debugf("Not adding unknown/unexpected type %s [%d] to operation", mqiToString("MQCFT", p.Type), p.Parameter)
		// Do nothing
	}
}

func max(a, b int) int {
	return int(math.Max(float64(a), float64(b)))
}
