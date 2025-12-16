#!/bin/bash

# Usage: http_compare.sh <url> <expected_file> [--insecure]
# Fetches the URL and compares the full body to the expected file. Exits 0 on a
# match so the scoreboard awards points.

set -euo pipefail

if [ $# -lt 2 ] || [ $# -gt 3 ]; then
    echo "Usage: $0 <url> <expected_file> [--insecure]"
    exit 1
fi

URL=$1
EXPECTED_FILE=$2
INSECURE=${3:-}

if [ ! -f "$EXPECTED_FILE" ]; then
    echo "Expected file not found: $EXPECTED_FILE"
    exit 1
fi

if ! command -v curl >/dev/null 2>&1; then
    echo "curl is required for this check."
    exit 1
fi

CURL_FLAGS=(-sSL --max-time 4)
if [ "$INSECURE" = "--insecure" ]; then
    CURL_FLAGS+=(--insecure)
fi

BODY_FILE=$(mktemp)
trap 'rm -f "$BODY_FILE"' EXIT

if ! curl "${CURL_FLAGS[@]}" "$URL" -o "$BODY_FILE"; then
    echo "Request to $URL failed"
    exit 1
fi

if cmp -s "$BODY_FILE" "$EXPECTED_FILE"; then
    exit 0
fi

echo "Content mismatch for $URL"
exit 1
