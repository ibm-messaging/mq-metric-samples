#!/bin/ksh

function waitForEnter {
  echo
  read dummy?"Press ENTER to continue"
  tput clear
}

curdir=`pwd`
. /opt/mqm/bin/setmqenv -k -n Installation1
inst=/opt/mqm

echo "Checking Cluster"

# Make sure we've got a clean configuration using OS mechanisms
port=4714
for qm in FR1 FR2 PR1 PR2 PR3 PR4
do
  (endmqm -i $qm
  dltmqm    $qm
  crtmqm -lf 64 $qm) >/dev/null 2>&1
  strmqm $qm 2>&1 | grep started


  runmqsc $qm << EOF >/dev/null 2>&1
ALTER QMGR  +
  SSLEV(ENABLED)     +
  CHLEV(DISABLED)     +
  CONFIGEV(DISABLED) +
  CMDEV(DISABLED)    +
  STRSTPEV(DISABLED) +
  LOCALEV(DISABLED)   +
  REMOTEEV(DISABLED)


* Don't waste time on an unnecessary service for this demo
STOP SERVICE(system.amqp.service) IGNSTATE(YES)
ALTER SERVICE(SYSTEM.AMQP.SERVICE) CONTROL(MANUAL)

ALTER QMGR CHLAUTH(DISABLED)

ALTER AUTHINFO(SYSTEM.DEFAULT.AUTHINFO.IDPWOS) AUTHTYPE(IDPWOS) +
  ADOPTCTX(YES) +
  CHCKLOCL(OPTIONAL) +
  CHCKCLNT(OPTIONAL)

ALTER QMGR CONNAUTH(SYSTEM.DEFAULT.AUTHINFO.IDPWOS)
REFRESH SECURITY

ALTER LISTENER(SYSTEM.DEFAULT.LISTENER.TCP) +
      TRPTYPE(TCP)   +
      PORT($port)     +
      CONTROL(QMGR)
START LISTENER(SYSTEM.DEFAULT.LISTENER.TCP)


EOF

  port=`expr $port + 1`
done

runmqsc FR1 << EOF >/dev/null 2>&1
  alter qmgr repos(CLUSTEST)
  def chl(TO.FR1) CHLTYPE(CLUSRCVR) TRPTYPE(TCP) +
    CONNAME('localhost(4714)') +
    CLUSTER(CLUSTEST)
  def chl(TO.FR2) CHLTYPE(CLUSSDR) TRPTYPE(TCP) +
    CONNAME('localhost(4715)') +
    CLUSTER(CLUSTEST)

EOF
runmqsc FR2 << EOF >/dev/null 2>&1
  alter qmgr repos(CLUSTEST)
  def chl(TO.FR2) CHLTYPE(CLUSRCVR) TRPTYPE(TCP) +
    CONNAME('localhost(4715)') +
    CLUSTER(CLUSTEST)
  def chl(TO.FR1) CHLTYPE(CLUSSDR) TRPTYPE(TCP) +
    CONNAME('localhost(4714)') +
    CLUSTER(CLUSTEST)

EOF

port=4716
for qm in PR1 PR2 PR3 PR4 
do
runmqsc $qm << EOF #>/dev/null 2>&1
  def chl(TO.FR1) CHLTYPE(CLUSSDR) TRPTYPE(TCP) +
    CONNAME('localhost(4714)') +
    CLUSTER(CLUSTEST)

  def chl(TO.$qm) CHLTYPE(CLUSRCVR) TRPTYPE(TCP) +
    CONNAME('localhost($port)') +
    CLUSTER(CLUSTEST)
EOF

clusQ="CLUS.WL.TEST"
if [ $qm != "PR1" ]
then
runmqsc $qm << EOF #>/dev/null 2>&1
  def QL($clusQ) CLUSTER(CLUSTEST) DEFPSIST(YES) DEFBIND(NOTFIXED)
EOF
fi
port=`expr $port + 1`
done

# get0 is my slightly modified version of amqsget, but the regular one will
# do fine here.
which get0 >/dev/null 2>&1
if [ $? -eq 0 ]
then
  get=get0
else
  get=$inst/samp/bin/amqsget
fi

echo Check comms works
echo "Hello from cluster at `date`" | $inst/samp/bin/amqsput $clusQ PR1
sleep 2
for qm in PR2 PR3 PR4
do
$get $clusQ $qm &
done
wait
sleep 1
tput clear


runmqsc FR1 << EOF
def qa(LOOP.ALIAS) TARGET(LOOP.INPUT)
def qr(LOOP.INPUT) RNAME(LOOP.RETURN) RQMNAME(FR2) XMITQ(FR2.XMIT)
def ql(LOOP.OUTPUT)
def ql(FR2.XMIT) usage(xmitq) TRIGGER TRIGTYPE(FIRST) INITQ(SYSTEM.CHANNEL.INITQ)

def chl(FR2.SDRCV) chltype(sdr) conname('localhost(4715)') xmitq(FR2.XMIT)
def chl(FR1.SDRCV) chltype(rcvr)
START CHL(FR2.SDRCV)

* A queue that goes nowhere so we can see some error cases. Deliberately
* no channel associated with it.
def qr(DEAD.END) RNAME(DEAD.END) RQMNAME(FR2) XMITQ(FR2.DEAD.XMIT)
def ql(FR2.DEAD.XMIT) usage(XMITQ)
EOF

runmqsc FR2 << EOF
def qr(LOOP.RETURN) RNAME(LOOP.OUTPUT) RQMNAME(FR1) XMITQ(FR1.XMIT)
def ql(FR1.XMIT) usage(xmitq) TRIGGER TRIGTYPE(FIRST) INITQ(SYSTEM.CHANNEL.INITQ)

def chl(FR1.SDRCV) chltype(sdr) conname('localhost(4714)') xmitq(FR1.XMIT)
def chl(FR2.SDRCV) chltype(rcvr)
START CHL(FR1.SDRCV)
EOF

sleep 2
echo "Hello from loop" | $inst/samp/bin/amqsput LOOP.ALIAS FR1
sleep 2
$get LOOP.OUTPUT FR1

