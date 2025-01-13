/*
 * This is an example of a Go program to format IBM MQ Event messages.
 * It is essentially a rewrite of the product's sample program amqsevta.c
 *
 * Functions in this file handle the command-line options
 */

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
package main

import (
	"bufio"
	"flag"
	"fmt"
	"os"
	"strconv"
	"strings"

	mq "github.com/ibm-messaging/mq-golang/v5/ibmmq"
	"golang.org/x/term"
)

// Command line options - explicit (uppercased-name) and derived
// (lowercased-name) values.
type Config struct {
	QMgrName  string
	QName     string
	TopicName string

	Output             string
	compact            bool
	jsonArray          bool
	WaitIntervalString string
	waitInterval       int32
	Browse             bool
	Decode             string
	rawString          bool
	rawNumber          bool
	ClientMode         bool
	UserId             string
	password           string

	Reconnect         string
	reconnectDisabled bool
	reconnectQMgr     bool
	reconnectAny      bool

	OTelEndpoint string
	OTelInsecure bool
}

var cf Config

// These are mostly the same flags as /opt/mqm/samp/bin/amqsevt
func initParms() {
	flag.StringVar(&cf.QMgrName, "m", "QM1", "Queue Manager name")
	flag.StringVar(&cf.QName, "q", "", "Queue Names (comma-separated)")
	flag.StringVar(&cf.TopicName, "t", "", "Topic Names (comma-separated)")

	flag.StringVar(&cf.Output, "o", "", "Additional format options: 'compact','array' (comma-separated)")
	flag.StringVar(&cf.Reconnect, "r", "", "Reconnection Option: d (disabled), m (qmgr), r (any)")
	flag.StringVar(&cf.Decode, "d", "r", "Decoded value format: r ('readable'), n (numbers), s (full MQI strings)")

	flag.StringVar(&cf.UserId, "u", "", "User ID")
	flag.BoolVar(&cf.Browse, "b", false, "Browse events")
	flag.BoolVar(&cf.ClientMode, "c", false, "Connect as client")
	flag.StringVar(&cf.WaitIntervalString, "w", "", "Wait time (seconds)")

	// New options to handle sending events direct to an OpenTelemetry endpoint
	flag.StringVar(&cf.OTelEndpoint, "e", "", "OTel Endpoint: 'stdout', 'http(s)://example.com:4318', 'example.com:4317'")
	flag.BoolVar(&cf.OTelInsecure, "i", false, "OTel Insecure connections allowed - certificate validation not required")

}

func parseParms() error {
	var err error

	flag.Parse()

	if len(flag.Args()) > 0 {
		err = fmt.Errorf("Unexpected additional command line parameters given.")
		fmt.Fprintf(flag.CommandLine.Output(), "Error: %v\n\n", err)
		flag.Usage() // This causes an immediate exit
	}

	if err == nil && cf.Output != "" {

		output := strings.ToLower(cf.Output)
		if strings.Contains(output, "compact") {
			cf.compact = true
		}
		if strings.Contains(output, "array") {
			cf.jsonArray = true
		}

		if !strings.Contains(output, "compact") && !strings.Contains(output, "array") {
			err = fmt.Errorf("Incorrect value for Output (-o) option")
			flag.Usage()
		}
	}

	if err == nil && cf.Reconnect != "" {
		switch cf.Reconnect {
		case "r":
			cf.reconnectAny = true
		case "m":
			cf.reconnectQMgr = true
		case "d":
			cf.reconnectDisabled = true
		default:
			err = fmt.Errorf("Incorrect value for Reconnect (-r) option")
			flag.Usage()
		}
	}

	if err == nil && cf.Decode != "" {
		switch cf.Decode {
		case "n":
			cf.rawNumber = true
		case "s":
			cf.rawString = true
		case "r": // Default - "readable"
			cf.rawString = false
			cf.rawNumber = false
		default:
			err = fmt.Errorf("Incorrect value for Decode (-d) option")
			flag.Usage()
		}
	}

	if err == nil && cf.UserId != "" {
		cf.password = getPassword("Enter password for " + cf.UserId + ": ")
	}

	if err == nil {
		if cf.WaitIntervalString == "" {
			cf.waitInterval = mq.MQWI_UNLIMITED
		} else {
			w, e := strconv.Atoi(cf.WaitIntervalString)
			if e != nil {
				err = fmt.Errorf("WaitInterval must be integer (seconds)")
			} else {
				cf.waitInterval = int32(w)
			}
		}
	}

	return err
}

func getPassword(prompt string) string {
	var password string

	fmt.Print(prompt)

	// Stdin is not necessarily 0 on Windows so call the os to try to find it
	bytePassword, err := term.ReadPassword(int(os.Stdin.Fd()))
	if err != nil {
		// Most likely cannot switch off echo (stdin not a tty)
		// So read from stdin instead
		scanner := bufio.NewScanner(os.Stdin)
		scanner.Scan()
		password = scanner.Text()

	} else {
		password = string(bytePassword)
		fmt.Println()
	}

	return strings.TrimSpace(password)
}
