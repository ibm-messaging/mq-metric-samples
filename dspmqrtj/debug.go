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
 * Various routines to help with debugging the program, including recursively dumping
 * received PCF groups and parameters into something readable
 */

import (
	"encoding/hex"
	"fmt"
	mq "github.com/ibm-messaging/mq-golang/v5/ibmmq"
	log "github.com/sirupsen/logrus"
	"strconv"
	"strings"
)

const (
	blank8  = "        "
	blank16 = blank8 + blank8
	blank64 = blank16 + blank16 + blank16 + blank16
)

// Format a buffer into readable blocks with a character equivalent alongside.
// For example:
//
//     0a000000 24000000 03000000 4b000000  |....$.......K...|
func formatBuf(b []byte) string {
	st := ""
	cst := ""
	s := ""
	w := 16 // line width
	lines := ""

	for i := 0; i < len(b); i++ {
		st = st + hex.EncodeToString(b[i:i+1])
		if i%4 == 3 {
			st = st + " "
		}
		if strconv.IsPrint(rune(b[i])) {
			s = string(rune(b[i]))
		} else {
			s = "."
		}
		cst = cst + s

		if i%w == (w - 1) {
			lines += st + " |" + cst + "|\n"
			st = ""
			cst = ""
		}
	}
	if len(st) > 0 {
		lines += (st + blank64)[0:36] + " |" + (cst + blank16)[0:w] + " |\n"
	}
	return lines
}

// The log.Debug routine doesn't handle multiple line input, and ignores "\n" breaks. So we do the splits here
// and send the lines individually
func logDebugMulti(str string) {
	if log.GetLevel() == log.DebugLevel {
		strSlice := strings.Split(str, "\n")
		for _, s := range strSlice {
			if strings.TrimSpace(s) != "" {
				log.Debug(s)
			}
		}
	}
}

// Turn a PCF parameter into a string. Not all PCF parm types
// are dealt with, just the ones I cared about for debugging this program
func pcfDebug(p *mq.PCFParameter) string {
	return pcfDebugNest(p, "", 0)
}
func pcfDebugNest(p *mq.PCFParameter, s string, depth int) string {
	pa := int(p.Parameter)
	val := ""

	parm := fmt.Sprintf("%d", pa)
	switch p.Type {
	case mq.MQCFT_STRING:
		parm = mq.MQItoString("CA", pa)
		val = p.String[0]
	case mq.MQCFT_INTEGER:
		parm = mq.MQItoString("IA", pa)
		if pa == int(mq.MQIACF_OPERATION_TYPE) {
			val = mq.MQItoString("OPER", int(p.Int64Value[0]))
		} else {
			val = fmt.Sprintf("%d", p.Int64Value[0])
		}
	case mq.MQCFT_BYTE_STRING:
		parm = mq.MQItoString("BACF", pa)
	case mq.MQCFT_GROUP:
		parm = mq.MQItoString("GACF", pa)
		val = fmt.Sprintf("%d", p.ParameterCount)
	}

	// Make sure we've got a NL on the end of the string. This gets stripped by log.Debug but not by this modules logDebugMulti function
	// Use the current depth to put spaces at the front of each output line
	s = s + fmt.Sprintf("%s Type: %-20.20s Parm: %s Val: %s\n", blank64[0:depth*2], mq.MQItoString("CFT", int(p.Type)), parm, val)

	// Recursively drill into groups, keeping track of the nesting depth. The complete formatted set of
	// parameters is built up on each return stage
	if p.Type == mq.MQCFT_GROUP {
		depth += 1
		switch int32(pa) {
		// Skip dumping large groups that I don't care about
		case mq.MQGACF_EMBEDDED_MQMD, mq.MQGACF_MQMD, mq.MQGACF_MESSAGE:
			// do nothing
		default:
			for i := 0; i < int(p.ParameterCount); i++ {
				s = pcfDebugNest(p.GroupList[i], s, depth)
			}
		}
	}

	return s
}

// Build some dummy data just to see how things look
func collectDummyData() {
	for i := 0; i < 2; i++ {
		act := new(Activity)
		addActivity(act, i)
		applName := fmt.Sprintf("Application_%d", i)
		act.ApplName = applName
		for j := 0; j < 2; j++ {
			op := newOperation(act)
			for k := 0; k < 2; k++ {
				key := fmt.Sprintf("%d.%d", i, j)
				op.Details[key] = "something"
			}
		}
	}
}
