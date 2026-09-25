#!/usr/bin/env python3

"""
Authenticate to a POP3 mailbox and ensure the mailbox is reachable.

Usage:
    pop3_check.py <server> <port> <username> <password> [--ssl]
"""

import argparse
import poplib
import socket
import sys


def main() -> int:
    parser = argparse.ArgumentParser(description="POP3 functional check")
    parser.add_argument("server")
    parser.add_argument("port", type=int)
    parser.add_argument("username")
    parser.add_argument("password")
    parser.add_argument("--ssl", action="store_true", help="Use POP3 over SSL")
    args = parser.parse_args()

    socket.setdefaulttimeout(5)

    try:
        if args.ssl:
            client = poplib.POP3_SSL(args.server, args.port, timeout=5)
        else:
            client = poplib.POP3(args.server, args.port, timeout=5)
        with client:
            client.user(args.username)
            client.pass_(args.password)
            # Listing messages is enough to assert functionality
            client.list()
    except Exception as exc:  # noqa: BLE001
        print(f"POP3 check failed: {exc}", file=sys.stderr)
        return 1

    print("POP3 check succeeded")
    return 0


if __name__ == "__main__":
    sys.exit(main())
