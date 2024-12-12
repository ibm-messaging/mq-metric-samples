package main

/*
  Copyright (c) IBM Corporation 2022, 2024

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
 * Read a password from stdin without echo
 */

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"golang.org/x/term"
)

func GetPasswordFromStdin(prompt string) string {
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

func GetPasswordFromFile(file string, removeFile bool) (string, error) {
	if file == "" {
		return "", nil
	}

	f, err := os.Open(file)
	if err != nil {
		err = fmt.Errorf("Opening file %s: %s", file, err.Error())
		return "", err
	}

	if removeFile {
		defer os.Remove(file)
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	scanner.Scan()
	p := scanner.Text()
	err = scanner.Err()
	if err != nil {
		err = fmt.Errorf("Reading file %s: %s", file, err)
	} else {
		p = strings.TrimSpace(p)
	}
	return p, err
}
