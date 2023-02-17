# This is an example of running one of the containers containing
# a collector program. You'll likely want to change this for your
# own setup, but it's a useful model of how to pass parameters.

. ./common.sh

if [ -z "$1" ]
then
  echo "Must provide a collector name such as 'mq_prometheus'"
  exit 1
fi

mon=$1
monbase=`echo $mon | sed "s/mq_//g"`

TAG=mq-metric-$monbase

# This line grabs a currently active IP address for this machine. It's probably
# not what you want to use for a real system but it's useful for testing.
addr=`ip addr | grep -v altname | grep -v docker | grep "state UP" -A2 | tail -n1 | awk '{print $2}' | cut -f1 -d'/'`
echo "Local address is $addr"
port="1414"

# Run the container for the designated monitor program, mounting a local
# instance of the configuration file. This one explicitly exposes the
# port that Prometheus is going to call in on.
docker run --rm -p 9190:9157 \
              -e MQSERVER="SYSTEM.DEF.SVRCONN/TCP/$addr($port)" \
              -v $OUTDIR/$mon.yaml:/opt/config/$mon.yaml \
              -it $TAG:$VER
