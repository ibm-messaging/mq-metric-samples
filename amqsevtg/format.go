/*
 * This is an example of a Go program to format IBM MQ Event messages.
 * It is essentially a rewrite of the product's sample program amqsevta.c
 *
 * Functions in this file handle the formatting of MQI constants, turning
 * them into something better-looking and suitable for JSON processing
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
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"unicode"

	mq "github.com/ibm-messaging/mq-golang/v5/ibmmq"
)

func printElem(elem *mq.PCFParameter, body map[string]interface{}) {
	var val int64
	var valS interface{}
	p := int(elem.Parameter)
	ps := mq.MQItoString("MQIA", p)
	if ps == "" {
		ps = mq.MQItoString("MQCA", p)
	}
	if ps == "" {
		ps = mq.MQItoString("MQBACF", p)
	}
	if ps == "" {
		ps = mq.MQItoString("MQGA", p)
	}
	psUnformatted := ps
	if ps == "" {
		ps = strconv.Itoa(p)
	} else {
		ps = prettyName(ps)
	}

	switch elem.Type {
	case mq.MQCFT_INTEGER:
		val = elem.Int64Value[0]

		if strings.HasSuffix(psUnformatted, "_OPTIONS") {
			m := make(map[string]any)
			m["value"] = val
			m["valueHex"] = fmt.Sprintf("0x%08X", val)
			arr := make([]string, 0)
			set := mq.PCFAttrToPrefix(elem.Parameter)

			if val == 0 {
				arr = append(arr, "None")
			} else {
				for i := 0; i < 32; i++ {
					p := (1 << uint(i))
					bf := int(val) & p
					if bf != 0 {
						arr = append(arr, prettyVal(mq.MQItoString(set, bf), val))
					}
				}
			}
			m["options"] = arr
			valS = m
		} else {
			switch elem.Parameter {
			case mq.MQIACF_REASON_CODE, mq.MQIACF_COMP_CODE:
				// These options are so common that we want to show both text and number
				nv := new(NV)
				nv.Name = prettyVal(mq.PCFValueToString(elem.Parameter, val), val)
				nv.Value = int32(val)
				valS = nv

			default:
				valS = mq.PCFValueToString(elem.Parameter, val)
				if _, err := strconv.Atoi(valS.(string)); err == nil {
					valS = val
				} else {
					valS = prettyVal(valS.(string), val)
				}
			}
		}

	case mq.MQCFT_INTEGER64:
		valS = elem.Int64Value[0]

	case mq.MQCFT_INTEGER_LIST:
		arr := make([]any, 0)
		set := mq.PCFAttrToPrefix(elem.Parameter)
		s := ""

		for i := 0; i < len(elem.Int64Value); i++ {
			v := int(elem.Int64Value[i])

			if strings.HasSuffix(psUnformatted, "_ATTRS") {
				if len(elem.Int64Value) == 1 && v == int(mq.MQIACF_ALL) {
					s = ATTRS_ALL
				} else {
					s = mq.MQItoString("MQIA", v)
					if s == "" {
						s = mq.MQItoString("MQCA", v)
					}
					if s == "" {
						s = mq.MQItoString("MQBACF", v)
					}
					if s == "" {
						s = strconv.Itoa(v)
					} else {
						s = prettyName(s)
					}
				}
			} else {
				s = prettyVal(mq.MQItoString(set, v), int64(v))
			}

			if s != "" {
				if _, err := strconv.Atoi(s); err == nil {
					arr = append(arr, v)
				} else {
					arr = append(arr, prettyVal(s, int64(v)))
				}
			} else {
				arr = append(arr, v)
			}
		}
		valS = arr

	case mq.MQCFT_INTEGER64_LIST:
		arr := make([]int64, 0)
		for i := 0; i < len(elem.Int64Value); i++ {
			v := elem.Int64Value[i]
			arr = append(arr, v)
		}
		valS = arr

	case mq.MQCFT_STRING:
		valS = elem.String[0]

	case mq.MQCFT_STRING_LIST:
		arr := make([]string, 0)
		for i := 0; i < len(elem.String); i++ {
			arr = append(arr, elem.String[i])
		}
		valS = arr

	case mq.MQCFT_BYTE_STRING:
		valS = strings.ToUpper(elem.String[0]) // already converted

	case mq.MQCFT_INTEGER_FILTER:
		ps = "filter"
		p := prettyName(mq.MQItoString("MQIA", int(elem.Filter.Parameter)))
		op := prettyName(mq.MQItoString("MQCFOP", int(elem.Filter.Operator)))
		valS = fmt.Sprintf("WHERE %s %s %d", p, op, elem.Filter.FilterValue.(int64))
	case mq.MQCFT_STRING_FILTER:
		ps = "filter"
		p := prettyName(mq.MQItoString("MQCA", int(elem.Filter.Parameter)))
		op := prettyName(mq.MQItoString("MQCFOP", int(elem.Filter.Operator)))
		valS = fmt.Sprintf("WHERE %s %s '%s'", p, op, elem.Filter.FilterValue.(string))
	case mq.MQCFT_BYTE_STRING_FILTER:
		ps = "filter"
		p := prettyName(mq.MQItoString("MQBACF", int(elem.Filter.Parameter)))
		op := prettyName(mq.MQItoString("MQCFOP", int(elem.Filter.Operator)))
		valS = fmt.Sprintf("WHERE %s %s '%s'", p, op, elem.Filter.FilterValue.(string))

	default:
		valS = "Unformatted type: " + mq.MQItoString("MQCFT", int(elem.Type))

	}
	body[ps] = valS
}

// Convert MQI constants corresponding to attribute names into something more readable.
// For example, "MQIA_RESOLVED_TYPE" becomes "resolvedType"
func prettyName(s string) string {
	if cf.rawString {
		return s
	}
	ss := []rune(s)
	rs := make([]rune, 0)

	foundFirstUnderscore := false
	afterUnderscore := false

	for i := 0; i < len(ss); i++ {
		c := ss[i]
		if foundFirstUnderscore {
			if c == '_' {
				afterUnderscore = true
				continue
			}

			if afterUnderscore {
				c = unicode.ToUpper(c)
				afterUnderscore = false
			} else {
				c = unicode.ToLower(c)
			}

			if c != '_' {
				rs = append(rs, c)
			}
		} else if c == '_' {
			foundFirstUnderscore = true
		}
	}
	return string(rs)
}

// Some strings still look better in all-upper-case. So we look for the
// converted mixed-case version and force them back into upper.
var forceUpper = []string{
	"Amqp",
	"Clwl",
	"Cpi",
	"Crl",
	"Csp",
	"Dns", /* Do this before "Dn" */
	"Dn",
	"Idpw",
	"Igq",
	"Ip ", /* note trailing space */
	"Ipv",
	"Ldap",
	"Lu62",
	"Mca ", /* note trailing space */
	"Mqi",
	"Mqsc",
	"Mr ",
	"Mru",
	"Pcf",
	"Sctq",
	"Ssl",
	"Tcp",
}

// Convert MQI constants corresponding to values into something more readable
// For example, "MQPER_NOT_PERSISTENT" becomes "Not Persistent"
func prettyVal(s string, v int64) string {
	return prettyValString(s, v, true)
}

// This underlying function lets us force the formatting to always return an MQI string regardless
// of a Decode=rawNumber command options. Useful for some of the header/control information where we're
// going to be printing both the number and string anyway.
func prettyValString(s string, v int64, rawNum bool) string {

	if cf.rawString {
		return s
	}
	if cf.rawNumber && rawNum {
		return strconv.FormatInt(v, 10)
	}

	if s == "MQOT_Q" {
		s = "Queue"
	}

	if !strings.Contains(s, "_") {
		return s
	}

	s = strings.ReplaceAll(s, "_Q_", "_QUEUE_")
	s = regexp.MustCompile("_Q$").ReplaceAllString(s, "_QUEUE")

	ss := []rune(s)
	rs := make([]rune, 0)

	foundFirstUnderscore := false
	nextUpper := true

	// Convert MQXXX_AAA_BBB to "Aaa Bbb"
	for i := 0; i < len(ss); i++ {
		c := ss[i]
		if foundFirstUnderscore {
			if c == '_' {
				c = ' '
				nextUpper = true
			}

			if nextUpper && c != ' ' {
				c = unicode.ToUpper(c)
				nextUpper = false
			} else {
				c = unicode.ToLower(c)
			}

			rs = append(rs, c)
		} else if c == '_' {
			foundFirstUnderscore = true
		}
	}

	// Create a string from the characters
	rc := string(rs)

	// Some strings look better with non-default upper/lowercase values
	for i := 0; i < len(forceUpper); i++ {
		rc = strings.ReplaceAll(rc, forceUpper[i], strings.ToUpper(forceUpper[i]))
	}

	// After converting to mixed-case and resetting a few strings, there
	// are still a small number of cases that look better with special
	// handling.
	rc = strings.ReplaceAll(rc, "Zos", "zOS")
	rc = strings.ReplaceAll(rc, " Os", " OS")
	rc = strings.ReplaceAll(rc, "_Os", "_OS")

	// Finally return the prettified-string
	return rc
}
