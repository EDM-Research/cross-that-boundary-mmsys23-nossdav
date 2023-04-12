#!/bin/bash

#initializing TC NETEM
reset_netem () {
	tc qdisc del dev eth0 root
	tc qdisc add dev eth0 root handle 1:0 netem delay 0ms loss 0%
	tc qdisc add dev eth0 parent 1:1 handle 10: tbf rate 4mbit buffer 5k limit 10k
	tc qdisc del dev eth1 root
	tc qdisc add dev eth1 root handle 1:0 netem delay 0ms loss 0%
	tc qdisc add dev eth1 parent 1:1 handle 10: tbf rate 4mbit buffer 5k limit 10k
	echo "Resetting all TC Netem configurations to client and server."
}


setNetwork () {
    echo "$1 $2 $3"

    tc qdisc change dev eth0 root handle 1:0 netem delay $1ms loss $3%
    tc qdisc change dev eth1 root handle 1:0 netem delay $1ms loss $3%
    tc qdisc change dev eth0 parent 1:1 handle 10: tbf rate $2kbit buffer 5k limit 10k
    tc qdisc change dev eth1 parent 1:1 handle 10: tbf rate $2kbit buffer 5k limit 10k
}

FILE="/logs/shaper_metrics.txt"
touch $FILE
reset_netem
echo "$(date +%s) SIMULATIONTHROUGHPUT 2000" >> $FILE
setNetwork 20 2000 0
sleep 10
echo "$(date +%s) SIMULATIONTHROUGHPUT 2000" >> $FILE
setNetwork 20 100 0
echo "$(date +%s) SIMULATIONTHROUGHPUT 100" >> $FILE
sleep 30
echo "$(date +%s) SIMULATIONTHROUGHPUT 100" >> $FILE
setNetwork 20 1000 0
echo "$(date +%s) SIMULATIONTHROUGHPUT 1000" >> $FILE
sleep 10
echo "$(date +%s) SIMULATIONTHROUGHPUT 1000" >> $FILE
setNetwork 20 100 0
echo "$(date +%s) SIMULATIONTHROUGHPUT 100" >> $FILE
sleep 30
echo "$(date +%s) SIMULATIONTHROUGHPUT 100" >> $FILE
setNetwork 20 2000 0
echo "$(date +%s) SIMULATIONTHROUGHPUT 2000" >> $FILE
