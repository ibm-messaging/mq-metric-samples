package errors

/*
  Copyright (c) IBM Corporation 2023

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
 * This package provides some generic error handling for
 * all of the collectors. Not much to start with, but there
 * may be some future convenient enhancements.
 */

import (
	"github.com/ibm-messaging/mq-golang/v5/ibmmq"
	"github.com/ibm-messaging/mq-golang/v5/mqmetric"

	log "github.com/sirupsen/logrus"
)

var (
	statusCollectionErrors = 0
)

const (
	maxStatusCollectionErrors = 3
)

// Allow (but still report) a small number of consecutive failures in collecting status. If that's
// exceeded, then exit with a fatal error
func HandleStatus(err error) {
	if err != nil {
		statusCollectionErrors++
		if statusCollectionErrors > maxStatusCollectionErrors {
			log.Fatalf("Error collecting status: %v. Maximum permitted failures reached.", err)
			if mqe, ok := err.(mqmetric.MQMetricError); ok {
				mqrc := mqe.MQReturn.MQRC
				if mqrc == ibmmq.MQRC_NO_MSG_AVAILABLE {
					log.Errorf("  Not all responses received in time. Perhaps queue manager is running slowly.")
				}
			}
		} else {
			log.Errorf("Error collecting status: %v. Continuing for now.", err)
		}
	} else {
		statusCollectionErrors = 0
	}
}
