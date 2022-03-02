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
 * This module handles command line parsing and the associated configuration attributes
 * Some of the flags are the same as the dspmqrte program, but a number are not. Use "-?" to
 * see them printed with the defaults and descriptions.
 */

import (
	"flag"
	"fmt"
	"github.com/ibm-messaging/mq-golang/v5/ibmmq"
	log "github.com/sirupsen/logrus"
	"strings"
	"time"
)

// Configuration attributes are loaded into this structure
type Config struct {
	QMgrName    string // -m
	TargetQ     string // -q
	TargetQMgr  string // -qm
	TargetTopic string // -ts
	ReplyQ      string // -rq
	realReplyQ  string

	CcdtUrl    string // -ccdtUrl
	ConnName   string // -connname
	Channel    string // -channel
	ClientMode bool

	Persistence int32
	Bind        int32
	OneLine     bool
	UserId      string
	Password    string

	MaxActivities int
	MaxWait       time.Duration
	Expiry        time.Duration
	MsgCount      int
	MsgLength     int

	// String versions that get parsed and validated before being converted into the real type
	PersistenceString string
	MaxWaitString     string
	ExpiryString      string
	BindString        string

	LogLevel string // loglevel
}

var cf Config

func initParms() {
	flag.StringVar(&cf.QMgrName, "m", "", "Queue Manager for connection")

	flag.StringVar(&cf.TargetQ, "q", "", "Target Queue")
	flag.StringVar(&cf.TargetQMgr, "qm", "", "Target Queue Manager")
	flag.StringVar(&cf.TargetTopic, "t", "", "Target Topic String")

	flag.StringVar(&cf.ReplyQ, "replyQ", "SYSTEM.DEFAULT.MODEL.QUEUE", "Reply Queue")

	flag.StringVar(&cf.CcdtUrl, "ccdtUrl", "", "CCDT Url")
	flag.StringVar(&cf.ConnName, "connName", "", "Connection Name (eg 'host.example.com(1414)')")
	flag.StringVar(&cf.Channel, "channel", "", "Channel")
	flag.BoolVar(&cf.ClientMode, "client", false, "Force connection as client")

	flag.StringVar(&cf.PersistenceString, "persist", "asdef", "Persistence (yes/no/asdef)")
	flag.StringVar(&cf.BindString, "bind", "asdef", "Cluster queue binding (open/notfixed/asdef)")

	flag.BoolVar(&cf.OneLine, "oneLine", false, "Produce JSON output on a single line")
	flag.StringVar(&cf.UserId, "user", "", "User Id")

	flag.IntVar(&cf.MaxActivities, "actCount", 100, "Maximum activities to collect")
	flag.StringVar(&cf.MaxWaitString, "maxWait", "30s", "Maximum Wait Time (duration)")
	flag.StringVar(&cf.ExpiryString, "expiry", "30s", "Expiry Time (duration)")
	flag.IntVar(&cf.MsgCount, "msgCount", 1, "Number of messages for testing cluster distributions")
	flag.IntVar(&cf.MsgLength, "msgLength", 0, "Additional KB for transmitted message")

	flag.StringVar(&cf.LogLevel, "logLevel", "Info", "Log Level")
}

func parseParms() error {
	var err error

	flag.Parse()

	err = verifyConfig()
	if err != nil {
		fmt.Fprintf(flag.CommandLine.Output(), "Error: %v\n\n", err)
		flag.Usage()
	} else if len(flag.Args()) > 0 {
		err = fmt.Errorf("Unexpected additional command line parameters given.")
		fmt.Fprintf(flag.CommandLine.Output(), "Error: %v\n\n", err)
		flag.Usage()
	}
	return err
}

func verifyConfig() error {
	var err error

	initLog()

	if err == nil {
		if cf.TargetTopic == "" && cf.TargetQ == "" {
			err = fmt.Errorf("One of queue or topic must be provided")
		}
	}

	if err == nil {
		cf.MaxWait, err = time.ParseDuration(cf.MaxWaitString)
		if err != nil {
			err = fmt.Errorf("Invalid value for max wait string parameter: %v", err)
		}
	}

	if err == nil {
		cf.Expiry, err = time.ParseDuration(cf.ExpiryString)
		if err != nil {
			err = fmt.Errorf("Invalid value for expiry parameter: %v", err)
		}
	}

	if err == nil {
		if cf.UserId != "" {
			cf.Password = GetPasswordFromStdin("Enter password for " + cf.UserId + ": ")
		}
	}

	if err == nil {
		if cf.PersistenceString != "" {
			switch strings.ToLower(cf.PersistenceString) {
			case "default", "asdef":
				cf.Persistence = ibmmq.MQPER_PERSISTENCE_AS_Q_DEF
			case "no":
				cf.Persistence = ibmmq.MQPER_NOT_PERSISTENT
			case "yes":
				cf.Persistence = ibmmq.MQPER_PERSISTENT
			default:
				err = fmt.Errorf("Invalid value for persistence: %s", cf.PersistenceString)
			}
		}
	}
	if err == nil {
		if cf.BindString != "" {
			switch strings.ToLower(cf.BindString) {
			case "default", "asdef":
				cf.Bind = ibmmq.MQOO_BIND_AS_Q_DEF
			case "notfixed":
				cf.Bind = ibmmq.MQOO_BIND_NOT_FIXED
			case "open":
				cf.Bind = ibmmq.MQOO_BIND_ON_OPEN
			default:
				err = fmt.Errorf("Invalid value for cluster queue binding: %s", cf.BindString)
			}
		}
	}

	log.Debugf("verifyConfig Config: %+v", cf)
	return err
}

func initLog() {
	level, err := log.ParseLevel(cf.LogLevel)
	if err != nil {
		level = log.InfoLevel
	}
	log.SetLevel(level)
}
