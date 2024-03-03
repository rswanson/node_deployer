#!/bin/bash

# Environment variables
NETWORK="mainnet"
DATA_DIR="/data/${NETWORK}/lighthouse"
# Start lighthouse
lighthouse bn \
  --network $NETWORK \
  --datadir $DATA_DIR \
  --http \
  --metrics \
  --disable-deposit-contract-sync \
  --checkpoint-sync-url https://mainnet.checkpoint.sigp.io 