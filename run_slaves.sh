#!/bin/bash

if [ -z "$MASTER" ] || [ -z "$EN" ] || [ -z "$KEY" ]; then
    echo "Error: MASTER and EN and KEY environment variables must be set"
    exit 1
fi

master=$MASTER
en=$EN
key=$KEY
cmd="./build/bin/klayslave --max-rps 5000 --master-host $master --master-port 5557 -key $key -tc="sessionTxTC" -endpoint $en --vusigned 1000 --batchSize 1"


# Extract the number from hostname and set base_sleep
hostname_suffix=$(hostname | grep -o '[0-9]*$')
if [ -n "$hostname_suffix" ]; then
    base_sleep=$((hostname_suffix * 10))
else
    base_sleep=0
fi

sudo ulimit -n 1048576

sudo sysctl -w net.ipv4.tcp_max_syn_backlog=65536
sudo sysctl -w fs.file-max=2097152
sudo sysctl -w net.core.netdev_max_backlog=250000
sudo sysctl -w net.core.rps_sock_flow_entries=32768
sudo sysctl -w net.core.rmem_max=16777216
sudo sysctl -w net.core.rmem_default=253952
sudo sysctl -w net.core.wmem_max=16777216
sudo sysctl -w net.core.wmem_default=253952
sudo sysctl -w net.core.somaxconn=65535
sudo sysctl -w net.ipv4.ip_local_port_range="1024    65535"
sudo sysctl -w net.ipv4.tcp_rmem="253952 253952 16777216"
sudo sysctl -w net.ipv4.tcp_wmem="253952 253952 16777216"
sudo sysctl -w net.ipv4.tcp_window_scaling=1
sudo sysctl -w net.ipv4.tcp_tw_reuse=1

sleep $base_sleep
echo "Executing first command: $cmd"
$cmd >> slave0.log 2>&1 &

echo "Executing second command: $cmd"
$cmd >> slave1.log 2>&1 &
tail -f slave1.log
