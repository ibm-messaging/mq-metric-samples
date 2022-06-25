#!/bin/bash

# © Copyright IBM Corporation 2021
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

# This script builds and runs a container with the designated metrics collector.
# It uses buildah+podman as an alternative approach to the Dockerfile mechanism shown
# elsewhere in this repository. It also uses the Red Hat Universal Base Images as
# the starting points.
#
# We first build the collector using a full-fat image. And then copy the compiled
# code to a slimmer container which can then be deployed. This script uses
# a combination of the supplied sample configuration file and some environment
# variables. You might prefer to keep the config file outside of the running container
# and just mount it into the system.

# Single parameter that can either be "CLEAN" or the name of one of the collectors.
# The default is to build the Prometheus module.
COLL="mq_prometheus"
clean=false
case "$1" in
  mq_*)
    COLL=$1
    ;;
  CLEAN)
    clean=true
    ;;
  *)
    if [ ! -z "$1" ]
    then
      echo "Only parameters are names of collectors (eg mq_prometheus) and 'CLEAN'"
      exit 1
    fi
    ;;
esac

# Set some variables.
ORG="github.com/ibm-messaging"
REPO="mq-metric-samples"
VRMF=9.3.0.0
GOVER=1.17.2
db=`echo $COLL | sed "s/mq_//g"`
#
imgName="mq-metric-$db"
imgNameRuntime=$imgName-runtime
imgNameBuild=$imgName-build

# This is a convenient way to tidy up old images, espcially after experimenting
if [ "$1" = "CLEAN" ]
then
  buildah list -a -n | grep ubi-working-container | awk '{print $1}' | xargs buildah rm  2>/dev/null
  buildah list -a -n | grep ubi-minimal-working-container | awk '{print $1}' | xargs buildah rm  2>/dev/null
  buildah list -a -n | grep $imgName | awk '{print $1}' | xargs buildah rm  2>/dev/null
  buildah images -n  | grep $imgName | awk '{print $3}' | xargs buildah rmi 2>/dev/null
  buildah images
  buildah list
  exit 0
fi

###########################################################################
# For normal operation, we start with a current UBI container. Unlike a
# Dockerfile build, the scripted builds rerun every step each time. They do not
# cache previous steps automatically.
###########################################################################
buildCtr=$(buildah from registry.access.redhat.com/ubi8/ubi)

# Install the Go package and a couple of other things. Failures here are going to be fatal
# so we check that we were at least able to get started
buildah run $buildCtr yum --disableplugin=subscription-manager -y install wget curl tar  gcc
if [ $? -ne 0 ]
then
 exit 1
fi
 
# The version of Go is not the default in the yum repositories so we get it directly from google download
# and make the links point at it
buildah config --env GOVER="go$GOVER" $buildCtr
buildah run $buildCtr /bin/bash -c 'cd /tmp && curl -LO "https://dl.google.com/go/$GOVER.linux-amd64.tar.gz" \
           && mkdir -p /usr/local \
           && tar -C /usr/local -xzf ./*.tar.gz \
           && rm -f /bin/go && ln -s /usr/local/go/bin/go /bin/go  \
           && rm -f ./*.tar.gz'
buildah run $buildCtr go version

# Set up the environment that's going to be needed to download the correct
# MQ client libraries and to strip out unneeded components from that package.
buildah config --env genmqpkg_incnls=1 \
               --env genmqpkg_incsdk=1 \
               --env genmqpkg_inctls=1 \
               --env RDURL="https://public.dhe.ibm.com/ibmdl/export/pub/software/websphere/messaging/mqdev/redist" \
               --env RDTAR="IBM-MQC-Redist-LinuxX64.tar.gz" \
               --env VRMF="$VRMF" \
               --env ORG="$ORG" \
               --env REPO="$REPO" \
                $buildCtr

# Get the MQ redistributable client downloaded and installed. Use the genmqpkg command
# to delete parts we don't need.
buildah run $buildCtr mkdir -p /opt/go/src/$ORG/$REPO /opt/bin /opt/config /opt/mqm
buildah run $buildCtr /bin/bash -c 'cd /opt/mqm \
                 && curl -LO "$RDURL/$VRMF-$RDTAR" \
                 && tar -zxf ./*.tar.gz \
                 && rm -f ./*.tar.gz \
                 && bin/genmqpkg.sh -b /opt/mqm'

# Copy over the source tree
buildah config --workingdir /opt/go/src/$ORG/$REPO $buildCtr
buildah copy  -q $buildCtr `pwd`/..

# The build process for the collector allows setting of some
# external variables, useful for debug and logging. Set them here...
buildStamp=`date +%Y%m%d-%H%M%S`
gitCommit=`git rev-list -1 HEAD --abbrev-commit 2>/dev/null`
if [ -z "$gitCommit" ]
then
  gitCommit="Unknown"
fi
hw=`uname -i`
os=`uname -s`
bp="$os/$hw"

# ... and use them as part of compiling the program. We actually do this
# by creating a script and copying it into the container where it gets run.
# That helps with wildcard expansion as it refers to the container rather than
# the local tree.
tmpfile=/tmp/build.sh.$$
cat << EOF > $tmpfile
cat config.common.yaml cmd/$COLL/config.collector.yaml > /opt/config/$COLL.yaml
go version
go build -mod=vendor -o /opt/bin/$COLL \
  -ldflags "-X \"main.BuildStamp=$buildStamp\" -X \"main.BuildPlatform=$bp\" -X \"main.GitCommit=$gitCommit\"" \
   cmd/$COLL/*.go
EOF

# Copy in the build command and remove it from the local machine. Then run it.
echo "Copying source"
buildah copy -q $buildCtr $tmpfile build.sh
rm -f $tmpfile
echo "Compiling program $COLL"
buildah run  $buildCtr /bin/bash ./build.sh
echo "Compilation finished"

# We now have a container image with the compiled code. Complete its generation with a 'commit'
echo "Comitting builder image"
buildah commit -q --squash --rm $buildCtr $imgNameBuild

###########################################################################
# Restart the image creation from a smaller base image
###########################################################################
runtimeCtr=$(buildah from registry.access.redhat.com/ubi8/ubi-minimal)

# Copy over the binaries that are going to be needed. Go doesn't have any
# separate runtime; it builds standalone programs. All we need to add is the
# MQ client code and the configuration file
echo "Copying built objects to runtime container"
buildah copy -q --from $imgNameBuild $runtimeCtr /opt/mqm /opt/mqm
buildah copy -q --from $imgNameBuild $runtimeCtr /opt/bin /opt/bin
buildah copy -q --from $imgNameBuild $runtimeCtr /opt/config /opt/config

buildah config --env IBMMQ_GLOBAL_CONFIGURATIONFILE=/opt/config/$COLL.yaml $runtimeCtr

# Complete the runtime container with an entrypoint
buildah config --entrypoint /opt/bin/$COLL $runtimeCtr
echo "Commiting runtime image"
buildah commit -q --squash --rm $runtimeCtr $imgNameRuntime

# Now run the image. The assumption is that you have a queue manager running on this machine on port 1414.
# But you can see how this run step can be modified. Using the environment variables overrides values in the
# configuration file which makes it easy to have a basic common config with only container-specific overrides
# provided via the env vars.
# We also map the port number to something different - the Prometheus engine would be configured to connect
# to 9158 even though the collector is listening on 9157 (the assigned number for MQ).
# Rather than having the config file embedded in the container image, you might prefer to mount it from a real local
# filesystem.

addr=`ip -4 addr | grep "state UP" -A2 | grep inet | tail -n1 | awk '{print $2}' | cut -f1 -d'/'`
if [ "$addr" = "" ]
then
  addr=`hostname`
fi

podman run \
    -e IBMMQ_GLOBAL_LOGLEVEL=DEBUG \
    -e IBMMQ_CONNECTION_CONNNAME=$addr \
    -e IBMMQ_CONNECTION_CHANNEL=SYSTEM.DEF.SVRCONN \
    -p 9158:9157 \
    $imgNameRuntime
