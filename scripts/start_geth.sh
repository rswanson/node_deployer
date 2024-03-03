#!/bin/bash

METRICS_ENDPOINT=http://127.0.0.1:8551
METRICS_PORT=6060
METRICS_ENABLED="true"
DATA_DIR="/data/mainnet/geth/data"
GETH_MAINNET=true
GETH_HOLESKY=false

# Start geth
/usr/local/bin/geth --datadir $DATA_DIR --metrics=$METRICS_ENABLED --metrics.addr $METRICS_ENDPOINT --metrics.port $METRICS_PORT 