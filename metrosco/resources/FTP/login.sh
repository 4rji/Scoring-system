#!/bin/bash

# Usage: login.sh <host> <username:password> [path]
# Authenticates to the FTP server and attempts to list the optional path.

set -euo pipefail

if [ $# -lt 2 ] || [ $# -gt 3 ]; then
    echo "Usage: $0 <host> <username:password> [path]"
    exit 1
fi

HOST=$1
CREDS=$2
TARGET_PATH=${3:-}

USER=${CREDS%%:*}
PASS=${CREDS#*:}

if [ -z "$USER" ] || [ -z "$PASS" ]; then
    echo "Credentials must be in the form username:password"
    exit 1
fi

LOG_FILE=$(mktemp)
trap 'rm -f "$LOG_FILE"' EXIT

FTP_COMMANDS=$(cat <<EOF
open $HOST
user $USER $PASS
$( [ -n "$TARGET_PATH" ] && echo "ls $TARGET_PATH" || echo "ls" )
quit
EOF
)

if ! echo "$FTP_COMMANDS" | timeout 4 ftp -inv >"$LOG_FILE" 2>&1; then
    echo "FTP connection to $HOST failed"
    exit 1
fi

if grep -Eqi "530 .*|Login failed|Not logged in" "$LOG_FILE"; then
    echo "FTP authentication failed for $USER@$HOST"
    exit 1
fi

if ! grep -Eqi "230 .*login" "$LOG_FILE"; then
    echo "FTP login did not complete successfully"
    exit 1
fi

exit 0
