#!/bin/bash

# Environment variables
DATA_DIR="/data/base"
RUST_LOG=info 



op-reth node \
    --chain base \
    --rollup.sequencer-http https://sequencer.base.org \
    --http \
    --ws \
    --authrpc.port 9551 \
    --authrpc.jwtsecret /data/shared/jwt.hex \
    --data-dir $DATA_DIR \
    --rpc.addr=127.0.0.1 \
    --rpc.port=7000 \


