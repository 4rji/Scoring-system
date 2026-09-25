#!/bin/bash

# Usage: banner.sh <host> <port> <regex>
# Connects to host:port and passes when the first line the server sends
# matches <regex> (e.g. '^SSH-' for SSH, '^220' for SMTP, '^\* OK' for IMAP).
# Only needs bash, no extra tools or credentials.

if [ $# -ne 3 ] || [ -z "$1" ] || [ -z "$2" ]; then
    echo "Usage: $0 <host> <port> <regex>"
    exit 1
fi

LINE=$(timeout 4 bash -c 'exec 3<>/dev/tcp/$0/$1 && read -t 3 -r line <&3 && echo "$line"' "$1" "$2" 2>/dev/null)

if [ -z "$LINE" ]; then
    echo "No banner from $1:$2"
    exit 1
fi

if ! echo "$LINE" | grep -Eq "$3"; then
    echo "Unexpected banner from $1:$2: $LINE"
    exit 1
fi

exit 0
