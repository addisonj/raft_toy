#!/bin/bash

go build
./raft_toy -bind 127.0.0.1:4000 -addr ":5000" -bootstrap -seed 127.0.0.1:4001,127.0.0.1:4002 -dir "/tmp/raft1" &
sleep 3
./raft_toy -bind 127.0.0.1:4001 -addr ":5001" -bootstrap -seed 127.0.0.1:4000,127.0.0.1:4002 -dir "/tmp/raft2" &
sleep 5
./raft_toy -bind 127.0.0.1:4002 -addr ":5002" -bootstrap -seed 127.0.0.1:4000,127.0.0.1:4001 -dir "/tmp/raft3" &

wait
