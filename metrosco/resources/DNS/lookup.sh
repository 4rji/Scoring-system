#!/bin/bash

# Usage: lookup.sh <record> [server]
# Performs a DNS lookup for the record (optionally using the specified server).

set -euo pipefail

if [ $# -lt 1 ] || [ $# -gt 2 ]; then
    echo "Usage: $0 <record> [server]"
    exit 1
fi

RECORD=$1
SERVER=${2:-}

if command -v dig >/dev/null 2>&1; then
    if [ -n "$SERVER" ]; then
        dig +time=2 "@$SERVER" "$RECORD" >/dev/null 2>&1 || exit 1
    else
        dig +time=2 "$RECORD" >/dev/null 2>&1 || exit 1
    fi
    exit 0
fi

if [ -n "$SERVER" ]; then
    nslookup "$RECORD" "$SERVER" >/dev/null 2>&1 || exit 1
else
    nslookup "$RECORD" >/dev/null 2>&1 || exit 1
fi

exit 0
