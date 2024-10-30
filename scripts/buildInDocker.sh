#!/bin/bash

# © Copyright IBM Corporation 2019,2020
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
# http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

# Script to build the monitor agent programs from within a Docker container

export PATH="${PATH}:/go/bin"
export CGO_CFLAGS="-I/opt/mqm/inc/"
export CGO_LDFLAGS_ALLOW="-Wl,-rpath.*"
export GOCACHE=/tmp/.cache

# Which monitor programs are to be built. By default, build the complete set available.
# It can be overridden by setting the value on the "docker run" command with
# a "-e MONITORS=..." flag.
if [ -z "$MONITORS" ]
then
  cd $GOPATH/src/$ORG/$REPO
  MONITORS=`ls cmd`
fi

echo "Using compiler:"
$GO version

# And do the builds into the output directory
cd $GOPATH/src/$ORG/$REPO
for m in $MONITORS
do
  srcdir=cmd/$m

  echo "Building $m"

  if [ ! -z "$BUILD_EXTRA_INJECT" ]
  then
    inject=`echo $BUILD_EXTRA_INJECT | sed "s/_/ /g"`
    $GO build -mod=vendor -o $GOPATH/out/$m -ldflags "$inject"   $srcdir/*.go
  else
    $GO build -mod=vendor -o $GOPATH/out/$m   $srcdir/*.go
  fi


  # Copy the supporting scripts into the output directory
  if [ -r $srcdir/$m.sh ]
  then
    cp $srcdir/*.sh $GOPATH/out
    chmod a+rx $GOPATH/out/*.sh
  fi
  if [ -r $srcdir/$m.mqsc ]
  then
    cp $srcdir/*.mqsc $GOPATH/out
  fi
  if [ -r $srcdir/config.collector.yaml ]
  then
    cat ./config.common.yaml $srcdir/config.collector.yaml > $GOPATH/out/$m.yaml
  fi

done
