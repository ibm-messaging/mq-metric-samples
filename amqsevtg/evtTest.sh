#!/bin/bash

function get0 {
    echo "CLEAR QL($1)" | runmqsc -e $2
}

qm=EVENTS
# These are the events queues + the other relevant queues used in the tests
qs="SYSTEM.ADMIN.PERFM.EVENT \
   SYSTEM.ADMIN.CHANNEL.EVENT\
   SYSTEM.ADMIN.QMGR.EVENT\
   SYSTEM.ADMIN.LOGGER.EVENT\
   SYSTEM.ADMIN.PUBSUB.EVENT\
   SYSTEM.ADMIN.CONFIG.EVENT\
   SYSTEM.ADMIN.COMMAND.EVENT\
   SYSTEM.ADMIN.STATISTICS.QUEUE\
   SYSTEM.ADMIN.ACCOUNTING.QUEUE\
   SYSTEM.ADMIN.TRACE.ACTIVITY.QUEUE\
   X\
   Y"

amqsput="/opt/mqm/samp/bin/amqsput"
# Use the product sample to start with, to check we are getting a good set of data
# amqsevt="/opt/mqm/samp/bin/amqsevt  -o json"

# This is the compiled program with some designated output options
# amqsevt="./amqsevtg -e localhost:4317 -i " # An OTel output path
# amqsevt="./amqsevtg -e stdout" # An OTel output path
amqsevt="./amqsevtg" # The default processing with JSON output

go build -o amqsevtg .
rc=$?
rm -rf /tmp/go-build*
if [ $rc -ne 0 ]
then
  echo "ERROR: Cannot build amqsevt"
  exit 1
fi

# endmqm -i $qm
strmqm $qm >/dev/null 2>&1

touch /var/mqm/qmgrs/$qm/mqat.ini

get0 X $qm >/dev/null
get0 Y $qm >/dev/null

for q in $qs
do
  get0 $q $qm >/dev/null
  echo "ALTER QL($q) MAXDEPTH(20) SHARE" | runmqsc -e $qm >/dev/null 2>&1
done

# Set options to ensure events are captured
# And run a few extra commands to check that Command/Config events are suitably formatted
cat << EOF | runmqsc -e $qm >/dev/null 2>&1
ALTER QMGR  CMDEV(ENABLED) +
  CONFIGEV(ENABLED) +
  PERFMEV(ENABLED)  +
  INHIBTEV(ENABLED) +
  LOCALEV(ENABLED) +
  AUTHOREV(ENABLED)

ALTER QMGR ACTVTRC(ON)
ALTER QMGR STATMQI(ON) STATCHL(HIGH) STATQ(ON)
ALTER QMGR ACCTMQI(ON)

DEF QL(X) PUT(DISABLED) REPLACE
DEF QL(Y) PUT(ENABLED) REPLACE

DISPLAY QMGR EVENT DESCR
DISPLAY QMGR ALL

DISPLAY QL(*) WHERE (CURDEPTH GT 0)
DISPLAY CONN(*) WHERE (APPLTAG LK 'amq*')

EOF

setmqaut -t qmgr -p mqguest -m $qm +connect

# Do some things to cause events

(
# "Put Disabled" so should cause event
echo hello | $amqsput X $qm
# This one ought to work
echo hello | $amqsput Y $qm
get0 Y $qm

# mqguest can connect but not open
echo hello | asid mqguest:mqguest $amqsput Y $qm
# app cannot even connect
echo hello | asid app:app         $amqsput Y $qm
) >/dev/null 2>&1

# Disable activity trace but force emission of stats/accounting info
cat << EOF | runmqsc -e $qm >/dev/null 2>&1
ALTER QMGR ACTVTRC(OFF)
RESET QMGR TYPE(STATISTICS)
EOF

log=/tmp/amqsevt.out
>$log

# Should now have a range of different types of events. The first execution
# will use the default standard of event queues. Later executions look for
# other types of event on their designated queues.
$amqsevt  -m $qm -w 1  | tee -a $log
$amqsevt  -m $qm -w 1 -q SYSTEM.ADMIN.STATISTICS.QUEUE  | tee -a $log
$amqsevt  -m $qm -w 1 -q SYSTEM.ADMIN.ACCOUNTING.QUEUE  | tee -a $log
$amqsevt  -m $qm -w 1 -q SYSTEM.ADMIN.TRACE.ACTIVITY.QUEUE  | tee -a $log

# Test that topics work as well - I know this particular qmgr will have programs running on my system
# But because we want to force exit, even if more events are arriving, we don't set a timeout on the command and
# instead fake an ENTER key after a fixed period.
(sleep 10;echo) | $amqsevt  -m QM1 -t '$SYS/MQ/INFO/QMGR/QM1/ActivityTrace/ApplName/#' | tee -a $log
