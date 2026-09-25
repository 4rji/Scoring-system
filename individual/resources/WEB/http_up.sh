#!/bin/bash

# Usage: http_up.sh <url>
# Passes when the web server answers with any status below 500.

if [ $# -ne 1 ] || [ -z "$1" ]; then
    echo "Usage: $0 <url>"
    exit 1
fi

if ! command -v curl &> /dev/null; then
    echo "This module requires curl to be installed."
    exit 1
fi

CODE=$(curl -sk -o /dev/null --max-time 4 -w '%{http_code}' "$1")

if [ "$CODE" = "000" ]; then
    echo "No website found at $1"
    exit 1
fi

if [ "$CODE" -ge 500 ]; then
    echo "Website at $1 returned $CODE"
    exit 1
fi

exit 0
