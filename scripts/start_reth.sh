#!/bin/bash

# Environment variables
INSTANCE_ID="1"
RETH_DATA_DIR="/data/${INSTANCE_ID}/reth"

RUST_LOG=info /data/bin/reth node --instance $INSTANCE_ID --datadir $RETH_DATA_DIR --authrpc.jwtsecret /data/shared/jwt.hex