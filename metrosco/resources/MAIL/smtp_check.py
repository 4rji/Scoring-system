#!/usr/bin/env python3

"""
Send a small probe email over SMTP to verify authentication and delivery path.

Usage:
    smtp_check.py <server> <port> <username> <password> <from> <to> [--starttls]
"""

import argparse
import email.utils
import smtplib
import socket
import sys
import time


def main() -> int:
    parser = argparse.ArgumentParser(description="SMTP functional check")
    parser.add_argument("server")
    parser.add_argument("port", type=int)
    parser.add_argument("username")
    parser.add_argument("password")
    parser.add_argument("from_addr")
    parser.add_argument("to_addr")
    parser.add_argument("--starttls", action="store_true", help="Use STARTTLS before login")
    args = parser.parse_args()

    socket.setdefaulttimeout(5)
    message_id = email.utils.make_msgid(domain=args.server)
    subject = f"Scoreboard probe {int(time.time())}"
    body = f"Probe message {message_id}"
    message = f"From: {args.from_addr}\r\nTo: {args.to_addr}\r\nSubject: {subject}\r\nMessage-ID: {message_id}\r\n\r\n{body}\r\n"

    try:
        with smtplib.SMTP(args.server, args.port, timeout=5) as client:
            client.ehlo()
            if args.starttls:
                client.starttls()
                client.ehlo()
            client.login(args.username, args.password)
            client.sendmail(args.from_addr, [args.to_addr], message)
    except Exception as exc:  # noqa: BLE001
        print(f"SMTP check failed: {exc}", file=sys.stderr)
        return 1

    print("SMTP check succeeded")
    return 0


if __name__ == "__main__":
    sys.exit(main())
