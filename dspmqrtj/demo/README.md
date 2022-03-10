# Demonstration script for dspmqrtj

This directory contains scripts to create and run a demo of dspmqrtj.

First run `crtClus.sh` which creates an MQ cluster containing 2 full
repositories and 4 partials. Various channels and queues are defined on
the queue managers. Port numbers from *4714* upwards are used for the
communication, which is all using `localhost`.

The demo itself first shows how the product-provided `dspmqrte` program
looks, and then it uses various options on `dspmqrtj` for comparison.
You can see how the JSON output is easily parsed using the `jq` filter.

The cluster has one queue defined on three of the partial qmgrs. The other
partial qmgr is where we trace from. First a single message is sent, which
shows which qmgr received the message. Then we send 10 messages in one
invocation of the program, using `jq` to extract the name of the final
queue manager in each pass, and then summarising how many went to each. It should
break down with 3/3/4 in some allocation.

Once you're done, the `dltClus.sh` script cleans up and removes the cluster.
