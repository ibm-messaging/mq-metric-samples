#!/bin/bash

#
#  Copyright (c) IBM Corporation 2022
#
#  Licensed under the Apache License, Version 2.0 (the "License");
#  you may not use this file except in compliance with the License.
#  You may obtain a copy of the License at
#
#  http://www.apache.org/licenses/LICENSE-2.0
#
#  Unless required by applicable law or agreed to in writing, software
#  distributed under the License is distributed on an "AS IS" BASIS,
#  WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
#  See the License for the specific language governing permissions and
#  limitations under the License.
#
#   Contributors:
#     Mark Taylor - Initial Contribution
#
# This program demonstrates the dspmqrtj program from the parent directory.
# It builds the program and then runs it with various options. The MQ C client
# must be installed in order for the compile to work. It also uses the "jq" program
# to parse JSON output.

function waitForEnter {
  echo
  read -p "Press ENTER to continue" x
}

# Make it look like you are typing the commands. The delay in here looks about right.
function slowPrint {
  for ff in $*
  do
    cat $ff | awk '{for(i=1;i<=length;i++){ printf "%c",substr($0,i,1); system("sleep 0.06");}; printf("\n"); system("sleep 2")}'
  done
}

which jq >/dev/null 2>&1
if [ $? -ne 0 ]
then
  echo "You need to install the jq program to run this demonstration"
  exit 1
fi

# Temporary file to hold commands so we can pretend we are typing them
f=/tmp/dspmqrtj.cmd

cd ..
mkdir bin 2>/dev/null

go build -mod=vendor -o bin/dspmqrtj *.go
if [ $? -ne 0 ]
then
  echo "Error building dspmqrtj program"
  exit 1
fi

cd ./bin


dspmq -m PR1 -n | grep RUNNING >/dev/null 2>&1
if [ $? -ne 0 ]
then
  echo "Need to build or start cluster."
  echo "Use the crtClus.sh command in this directory."
  exit 1
fi


# First show the product-supplied program.
tput clear
export MQSERVER="SYSTEM.DEF.SVRCONN/TCP/localhost(4714)"
cmd="dspmqrte -c -m FR1 -q LOOP.ALIAS -v outline"
echo "\$ $cmd" > $f
slowPrint $f
eval $cmd 2>&1| more
waitForEnter
unset MQSERVER

# Now show how the JSON version looks doing the same thing
tput clear
cmd="dspmqrtj -m FR1 -q LOOP.ALIAS -channel SYSTEM.DEF.SVRCONN -connName 'localhost(4714)' -maxWait 3s"
echo "\$ $cmd" > $f
slowPrint $f
sleep 1
eval $cmd | more
waitForEnter

# Now show how the JSON version looks doing the same thing again. But as a single line, preferred by some tools
#tput clear
#cmd="dspmqrtj -m FR1 -q LOOP.ALIAS -channel SYSTEM.DEF.SVRCONN -connName 'localhost(4714)' -maxWait 3s -oneLine"
#echo "\$ $cmd" > $f
#slowPrint $f
#sleep 1
#eval $cmd | more
#waitForEnter

# Use JQ to extract just the operations from doing the same thing
tput clear
cmd="dspmqrtj -m FR1 -q LOOP.ALIAS -channel SYSTEM.DEF.SVRCONN -connName 'localhost(4714)' -maxWait 3s | jq -r '.activities[].operations[].operation'"
echo "\$ $cmd" > $f
slowPrint $f
eval $cmd | more
waitForEnter

# Show how an unavailable channel stops the flow
#tput clear
#cmd="dspmqrtj -m FR1 -q DEAD.END -channel SYSTEM.DEF.SVRCONN -connName 'localhost(4714)' -maxWait 3s"
#echo "\$ $cmd" > $f
#slowPrint $f
#eval $cmd | more
#waitForEnter

# And a real error
tput clear
cmd="dspmqrtj -m FR1 -q Q.NOT.DEFINED -channel SYSTEM.DEF.SVRCONN -connName 'localhost(4714)' -maxWait 3s"
echo "\$ $cmd" > $f
slowPrint $f
eval $cmd | more
waitForEnter


# A single message in the cluster. You can see where it ends up from the finalQueueManager value
tput clear
cmd="dspmqrtj -m PR1 -q CLUS.WL.TEST -maxWait 3s -msgCount 1 -bind notfixed"
echo "\$ $cmd" > $f
slowPrint $f
eval $cmd | more
waitForEnter

# Send a set of messages, and see how they are distributed across the cluster. The JQ filter extracts the
# final destination and then we sort/count them.
tput clear
cmd="dspmqrtj -m PR1 -q CLUS.WL.TEST -maxWait 3s -msgCount 10 -bind notfixed | jq -r '.summary.finalQueueManager' | sort | uniq -c"
echo "\$ $cmd" > $f
slowPrint $f
eval $cmd
