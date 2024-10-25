package main

/*
  Copyright (c) IBM Corporation 2021

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
This file manages the state between different threads of the collector. It exposes
the state via getter/setter functions which use atomic read/write operations. We also
map between integer values internally and boolean state which is all we really care about.
*/

import (
	"sync/atomic"
)

// These are really booleans but I'm using atomic updates
// to avoid potential races (though they'd likely be harmless) between
// the main code and the callbacks
type status struct {
	connectedOnce   int32
	connectedQMgr   int32
	collectorEnd    int32
	collectorSilent int32
	firstCollection int32
}

var (
	st status
)

func isConnectedQMgr() bool {
	b := atomic.LoadInt32(&st.connectedQMgr)
	return b != 0
}

func setConnectedQMgr(b bool) {
	if b {
		atomic.StoreInt32(&st.connectedQMgr, 1)
	} else {
		atomic.StoreInt32(&st.connectedQMgr, 0)
	}
}

func isCollectorEnd() bool {
	b := atomic.LoadInt32(&st.collectorEnd)
	return b != 0
}

func setCollectorEnd(b bool) {
	if b {
		atomic.StoreInt32(&st.collectorEnd, 1)
	} else {
		atomic.StoreInt32(&st.collectorEnd, 0)
	}
}

func isFirstCollection() bool {
	b := atomic.LoadInt32(&st.firstCollection)
	return b != 0
}

func setFirstCollection(b bool) {
	if b {
		atomic.StoreInt32(&st.firstCollection, 1)
	} else {
		atomic.StoreInt32(&st.firstCollection, 0)
	}
}

func isCollectorSilent() bool {
	b := atomic.LoadInt32(&st.collectorSilent)
	return b != 0
}

func setCollectorSilent(b bool) {
	if b {
		atomic.StoreInt32(&st.collectorSilent, 1)
	} else {
		atomic.StoreInt32(&st.collectorSilent, 0)
	}
}

func setConnectedOnce(b bool) {
	if b {
		atomic.StoreInt32(&st.connectedOnce, 1)
	} else {
		atomic.StoreInt32(&st.connectedOnce, 0)
	}
}

func isConnectedOnce() bool {
	b := atomic.LoadInt32(&st.connectedOnce)
	return b != 0
}
