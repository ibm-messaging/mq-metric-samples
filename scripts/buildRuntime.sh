#!/bin/bash
# © Copyright IBM Corporation 2020
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
#
# This script will build a container that holds the runtime for a collector

. ./common.sh

curdir=`pwd`

# Default is to build all collectors. Can use command parameters to only
# build a subset
if [ -z "$1" ]
then
  monitors=`ls cmd`
else
  monitors="$*"
fi

# Compile the programs
cd scripts
 ./buildMonitors.sh $monitors
cd ..

if [ $? -ne 0 ]
then
  echo "Building the monitors failed"
  exit 1
fi

# Copy the Dockerfile into the same directory as the built programs for convenience
cp $curdir/Dockerfile.run $OUTDIR
for mon in $monitors
do
  monbase=`echo $mon | sed "s/mq_//g"`
  TAG=mq-metric-$monbase
  cd $OUTDIR
  docker build --build-arg MONITOR_ARG=$mon -f Dockerfile.run -t $TAG:$VER .
  rc=$?
done
rm -f $OUTDIR/Dockerfile.run
