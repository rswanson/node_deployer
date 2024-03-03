#!/bin/bash

EE_ENDPOINT=http://127.0.0.1:8551
EE_JWT_SECRET_FILE="/data/shared/jwt.hex"
METRICS_ENABLED="true"
REST_API_ENABLED="true"
DATA_DIR="/data/mainnet/teku/data"
CHECKPOINT_SYNC_URL="https://beaconstate.ethstaker.cc"

# Start Teku
/data/repos/teku/build/install/teku/bin/teku --ee-endpoint="${EE_ENDPOINT}" --ee-jwt-secret-file="$EE_JWT_SECRET_FILE" --metrics-enabled=$METRICS_ENABLED --rest-api-enabled=$REST_API_ENABLED --checkpoint-sync-url="$CHECKPOINT_SYNC_URL" --data-path=$DATA_DIR