#!/bin/bash

# Environment variables
NETWORK="mainnet"
DATA_DIR="/data/${NETWORK}/nimbus"
METRICS_ENABLED="true"

# Start nimbus
/data/repos/nimbus2-eth/build/nimbus_beacon_node \
  --network $NETWORK \
  --data-dir $DATA_DIR \
  --metrics $METRICS_ENABLED