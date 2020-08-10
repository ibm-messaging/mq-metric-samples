# Common startup for all the build/run scripts

function latestSemVer {
  (for x in $*
  do
    echo $x | sed "s/^v//g"
  done) | sort -n | tail -1
}

# Go to the root of the repo
cd ..

# Assume repo tags have been created in a sensible order. Find the mq-golang
# version in the dep file (it's the only dependency explicitly listed for now)
# and the current Git tag for this repo. Then pick the latest version to create
# the Docker tag
VERDEP=`cat go.mod | awk '/mq-golang/ {print $2}' `
VERREPO=`git tag -l | sort | tail -1 `

VER=`latestSemVer $VERDEP $VERREPO`
if [ -z "$VER" ]
then
  VER="latest"
fi

# echo "VERDEP=$VERDEP VERREPO=$VERREPO"

# This is the directory where the binary and config files live
OUTDIR=$HOME/tmp/mq-metric-samples/bin
