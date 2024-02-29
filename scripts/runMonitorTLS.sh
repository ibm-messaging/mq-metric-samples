#!/bin/bash
# © Copyright IBM Corporation 2022
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
# This script builds a container that holds the runtime for a collector and
# then runs it with TLS options for a secure connection to the queue manager

origDir=`pwd`

. ./common.sh

monitor="mq_prometheus"

# Compile the program
cd $origDir
export MONITORS=$monitor
$origDir/buildMonitors.sh $monitor
if [ $? -ne 0 ]
then
  echo "Building the $monitor monitor failed"
  exit 1
fi
cd $rootDir

# Copy the Dockerfile into the same directory as the built programs for convenience
cp $rootDir/Dockerfile.run $OUTDIR

monbase=`echo $monitor | sed "s/mq_//g"`
TAG=mq-metric-$monbase

cd $OUTDIR
docker build --build-arg MONITOR_ARG=$monitor -f Dockerfile.run -t $TAG:$VER .
rc=$?
rm -f $OUTDIR/Dockerfile.run

# This line grabs a currently active IP address for this machine. It's probably
# not what you want to use for a real system but it's useful for testing.
addr=`ip addr | grep -v altname | grep "state UP" -A2 | grep inet | grep -v inet6 | tail -n1 | awk '{print $2}' | cut -f1 -d'/'`
echo "Local address is $addr"
port="1414"

# Create the YAML configuration file. The default does not set client connection, so
# force that
tlsYaml=$OUTDIR/$monitor.tls.yaml
rm -f $tlsYaml
cat $OUTDIR/$monitor.yaml | sed "s/clientConnection:/clientConnection: true/g" > $tlsYaml  
chmod 555 $tlsYaml

# Create the ccdt file with a suitable address as "localhost" is problematic in containers
ccdtJson=$OUTDIR/ccdt.json
rm -f $ccdtJson
cat $origDir/ccdt.json | sed "s/localhost/$addr/g" > $ccdtJson
chmod 555 $ccdtJson

# Clean the directory that will contain certificates
rm -f $OUTDIR/ssl/*
mkdir -p $OUTDIR/ssl 2>/dev/null

# The TLS configuration in this example is built from a local queue manager. It's
# a very simple self-signed cert, and only one-way TLS is required. For two-way, you'd
# have to obtain a suitable private key and certificate for the client and install it into the keystore
# with a label.
# 
# You might prefer to copy certificates into the container and then rebuild the
# keystore as part of the Dockerfile or the startup script. The keystore is created from scratch
# into a local directory along with associated stash file etc. The whole directory can then be mounted
# into the container.
runmqakm -keydb -create -db $OUTDIR/ssl/key.kdb -pw passw0rd -stash -type kdb
# Extract the queue managers public cert
runmqakm -cert -extract -db /var/mqm/ssl/key.kdb -stashed -label "ibmwebspheremqqm1" -target $OUTDIR/ssl/qm1.pem -format ascii
# And import it to the client's keystore
runmqakm -cert  -import -target $OUTDIR/ssl/key.kdb -stashed -file $OUTDIR/ssl/qm1.pem
rm -f $OUTDIR/ssl/qm1.pem

# Run the container for the monitor program, mounting a local
# instance of the configuration file. This one explicitly exposes the
# port that Prometheus is going to call in on. We also mount the TLS keystore and CCDT. 
# Again, these are resources that you might prefer to copy into the container rather than mounting from 
# a local version.
docker run --rm -p 9190:9157 \
              -v $OUTDIR/$monitor.tls.yaml:/opt/config/$monitor.yaml \
              -v $OUTDIR/ccdt.json:/opt/config/ccdt.json \
              -v $OUTDIR/ssl:/var/mqm/ssl \
              -e IBMMQ_GLOBAL_LOGLEVEL=INFO \
              -e MQSSLKEYR=/var/mqm/ssl/key \
              -e MQCHLLIB=/opt/config -e MQCHLTAB=ccdt.json \
              -it $TAG:$VER

