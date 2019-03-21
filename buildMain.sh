# © Copyright IBM Corporation 2019
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

# This simple script builds a Docker container whose purpose is simply
# to compile the binary components of the monitoring programs, and then to copy those
# programs to a local temporary directory.
GOPATH="/go"

TAG="mq-metric-samples-gobuild"
# Assume repo tags have been created in a sensible order
VER=`git tag -l | sort | tail -1 | sed "s/^v//g"`
if [ -z "$VER" ]
then
  VER="latest"
fi
echo "Building container $TAG:$VER"

# Build a container that has all the pieces needed to compile the Go programs for MQ
docker build --build-arg GOPATH_ARG=$GOPATH -t $TAG:$VER .
rc=$?

if [ $rc -eq 0 ]
then
  # Run the image to do the compilation and extract the files
  # from it into a local directory mounted into the container.
  OUTDIR=$HOME/tmp/mq-metric-samples/bin
  rm -rf $OUTDIR
  mkdir -p $OUTDIR

  # Get some variables to pass the build information into the compile steps
  buildStamp=`date +%Y%m%d-%H%M%S`
  gitCommit=`git rev-list -1 HEAD --abbrev-commit`

  # Set this for any special status
  extraInfo=""

  # Add "-e MONITORS=..." to only compile a subset of the monitor programs
  # Mount an output directory
  # Delete the container once it's done its job
  docker run --rm \
          -v $OUTDIR:$GOPATH/bin:z \
          -e BUILD_EXTRA_INJECT="-X \"main.BuildStamp=$buildStamp $extraInfo\" -X \"main.GitCommit=$gitCommit\"" \
          $TAG:$VER
  echo "Compiled programs should now be in $OUTDIR"        
fi
