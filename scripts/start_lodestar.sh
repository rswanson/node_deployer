#!/bin/bash

# Environment variables
NETWORK="mainnet"
DATA_DIR="/data/${NETWORK}/lodestar"
# Start lodestar
lodestar \
  --network $NETWORK \
  --datadir $DATA_DIR \
  --http \
  --metrics \
  --disable-deposit-contract-sync 
