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
	mq "github.com/ibm-messaging/mq-golang/v5/ibmmq"
	log "github.com/sirupsen/logrus"
)

type Operation struct {
	OperationString string                 `json:"operation,omitempty"`
	operationVal    int32                  `json:"-"`
	Date            string                 `json:"date,omitempty"`
	Time            string                 `json:"time,omitempty"`
	TimeStampEpoch  int64                  `json:"epochMilliSec,omitempty"`
	Details         map[string]interface{} `json:"details,omitempty"`
}

//type TraceRoute struct {
//	Details   map[string]interface{} `json:"details,omitempty"`
//}

type Activity struct {
	ApplName    string                 `json:"applName"`
	ApplType    string                 `json:"applType"`
	Description string                 `json:"description"`
	Operations  []*Operation           `json:"operations,omitempty"`
	TraceRoute  map[string]interface{} `json:"traceRoute,omitempty"`
}

type Info struct {
	Parms              string `json:"parameters"`
	Success            string `json:"success,omitempty"`
	Failure            string `json:"failure,omitempty"`
	TotalTime          int64  `json:"totalTimeMilliSec"`
	LastTime           string `json:"-,omitempty"`
	LastDate           string `json:"-,omitempty"`
	LastTimeStampEpoch int64  `json:"-,omitempty"`

	ChannelCount int    `json:"channelCount,omitempty"` // Zero is considered "empty" when using JSON Marshal
	FinalQMgr    string `json:"finalQueueManager,omitempty"`
}

type Output struct {
	Info             Info                   `json:"summary"`
	Error            map[string]interface{} `json:"error,omitempty"`
	FirstActivity    *Activity              `json:"-"` // For cluster testing, this is only fully filled on the first iteration
	immediateFailure bool
	Activities       []*Activity `json:"activities,omitempty"`
}

var (
	o               Output
	activityMap     = make(map[int]*Activity)
	highestActivity = 0
)

// Reset variables for each loop when we're testing cluster distributions
func initActivity() {
	for k, _ := range activityMap {
		delete(activityMap, k)
	}
	o.Error = make(map[string]interface{})
	highestActivity = 0
	seenDiscard = false
}

func addErr(d string, e error) {
	o.Error = make(map[string]interface{})
	o.Error[STR_DESCRIPTION] = d
	if e != nil {
		o.Error[STR_DETAILED_ERROR] = e.Error()
		if mqret, ok := e.(*mq.MQReturn); ok {
			o.Error[STR_MQRC] = mqret.MQRC
			o.Error[STR_MQCC] = mqret.MQCC
		}
		//log.Error(e)
	}
}

func newActivity() *Activity {
	act := new(Activity)
	act.TraceRoute = make(map[string]interface{})
	return act
}

func addActivity(act *Activity, i int) {
	if i == 0 {
		o.FirstActivity = act // Stash the first one so it's preserved across loops
	} else {
		activityMap[i] = act
	}
	highestActivity = max(highestActivity, i)
	log.Debugf("Adding activity at position %d", i)
}

func newOperation(act *Activity) *Operation {
	op := new(Operation)
	op.Details = make(map[string]interface{})
	act.Operations = append(act.Operations, op)
	log.Debug("Creating a new Operation object")
	return op
}
