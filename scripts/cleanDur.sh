#!/bin/bash

# This script will delete all durable subscriptions for a given
# queue manager that begin with a specified prefix.
# It should be used whenever you want to clean up after a collector
# program has been used, which uses the durable subscription configuration.
# It should also be used after any configuration change has been applied to the
# collector, before you restart the collector.
qm=$1
prefix=$2

if [ -z "$qm" -o -z "$prefix" ]
then
  echo "Usage: cleanDur.sh qmgr prefix"
  exit 1
fi

. setmqenv -m $qm -k

# The collectors build the subscription name based on the prefix followed by underscore. We can't
# delete based on a wildcard, so list them all and delete individually
filter="$prefix"_"*"
(echo "DIS SUB('$filter') WHERE (DURABLE EQ YES)" | runmqsc $qm | grep "Monitor" |\
    sed "s/(/ /g" | sed "s/)/ /g" | awk '{print $2}' |\
 while read line
do
  echo "DELETE SUB('$line')"
done) | runmqsc $qm
