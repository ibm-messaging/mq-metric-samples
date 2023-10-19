package config

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

// This reports the version of the mq-golang dependency that is pulled in.
import (
	"runtime/debug"
	//"fmt"
	"strings"
)

func MqGolangVersion() string {
	mqGolangVersion := "Unknown"

	bi, ok := debug.ReadBuildInfo()
	if ok {
		//fmt.Printf("BuildInfo: %v\n", bi)

		for _, mod := range bi.Deps {
			if strings.Contains(mod.Path, "ibm-messaging/mq-golang") {
				mqGolangVersion = mod.Version
			}
			//fmt.Printf("Module: %s Version: %s\n", mod.Path, mod.Version)
		}
	}

	return mqGolangVersion
}
