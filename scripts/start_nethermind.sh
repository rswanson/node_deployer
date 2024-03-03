#!/bin/bash

# Environment variables
NETWORK="mainnet"
DATA_DIR="/data/${NETWORK}/nethermind"
METRICS_ENABLED="true"

# Start nethermind
nethermind \
  --config $NETWORK \
  --baseDbPath $DATA_DIR \
  --JsonRpc.Enabled true \
  --JsonRpc.Host 127.0.0.1 \
  --JsonRpc.Port 8545 \
  --metrics $METRICS_ENABLED