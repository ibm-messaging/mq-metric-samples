#!/bin/bash

. /opt/mqm/bin/setmqenv -n Installation1 -k

for qm in FR1 FR2 PR1 PR2 PR3 PR4
do
   endmqm -i $qm
   dltmqm    $qm
done
