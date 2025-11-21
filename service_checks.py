"""
Service verification logic derived from sys.conf.

Includes the scoring checks used for:
1. HTTP (port 80) - specific request with expected content.
2. HTTPS (port 443) - same as HTTP but over SSL with exact content match.
3. SMTP - sending and receiving mail from a valid account.
4. POP3 - AD users connect, run commands, and validate responses.
5. FTP - correct authentication and file access.
6. DNS - correct domain resolution.

The data structures at the end reproduce the values present in sys.conf so
they can be reused by a future web app without re-parsing the file.
"""

from __future__ import annotations

from dataclasses import dataclass, field
from typing import Iterable, List, Optional, Sequence, Tuple
import ftplib
import hashlib
import http.client
import io
import poplib
import re
import smtplib
import socket
import ssl
import urllib.parse


@dataclass
class CheckResult:
    ok: bool
    detail: str


@dataclass
class HTTPCheck:
    host: str
    port: int = 80
    path: str = "/"
    expected_content: Optional[str] = None
    use_regex: bool = False
    expected_status: int = 200
    scheme: str = "http"
    username_param: Optional[str] = None
    password_param: Optional[str] = None
    username: Optional[str] = None
    password: Optional[str] = None


@dataclass
class SMTPCheck:
    host: str
    port: int = 25
    sender: str = ""
    receiver: str = ""
    body: str = ""


@dataclass
class POP3Command:
    command: str
    expected: str
    use_regex: bool = False


@dataclass
class POP3Check:
    host: str
    port: int = 110
    username: str = ""
    password: str = ""
    use_ssl: bool = False
    commands: List[POP3Command] = field(default_factory=list)


@dataclass
class FTPFileExpectation:
    name: str
    sha256: Optional[str] = None
    regex: Optional[str] = None


@dataclass
class FTPCheck:
    host: str
    port: int = 21
    username: str = "anonymous"
    password: str = "anonymous@"
    files: List[FTPFileExpectation] = field(default_factory=list)


@dataclass
class DNSRecordExpectation:
    kind: str
    domain: str
    answers: Sequence[str]


@dataclass
class DNSCheck:
    server: str
    port: int = 53
    records: List[DNSRecordExpectation] = field(default_factory=list)


class ServiceChecker:
    def __init__(self, timeout: int = 5) -> None:
        self.timeout = timeout
        self._ssl_context = ssl.create_default_context()

    def http(self, check: HTTPCheck) -> CheckResult:
        conn_cls = (
            http.client.HTTPSConnection if check.scheme.lower() == "https" else http.client.HTTPConnection
        )
        conn_kwargs = {"timeout": self.timeout}
        if check.scheme.lower() == "https":
            conn_kwargs["context"] = self._ssl_context
        conn = conn_cls(check.host, check.port, **conn_kwargs)

        params = {}
        if check.username_param and check.username is not None:
            params[check.username_param] = check.username
        if check.password_param and check.password is not None:
            params[check.password_param] = check.password

        path = check.path or "/"
        if params:
            separator = "&" if "?" in path else "?"
            path = f"{path}{separator}{urllib.parse.urlencode(params)}"

        try:
            conn.request("GET", path)
            resp = conn.getresponse()
            body = resp.read().decode("utf-8", errors="replace")
        except Exception as exc:
            return CheckResult(False, f"http error: {exc}")
        finally:
            conn.close()

        if check.expected_status and resp.status != check.expected_status:
            return CheckResult(False, f"status {resp.status} != expected {check.expected_status}")

        if check.expected_content:
            if check.use_regex:
                matched = re.search(check.expected_content, body, re.IGNORECASE | re.MULTILINE | re.DOTALL)
                if not matched:
                    return CheckResult(False, "body does not match expected regex")
            else:
                if check.expected_content not in body:
                    return CheckResult(False, "body does not contain expected text")

        return CheckResult(True, f"{check.scheme.upper()} check passed")

    def https(self, check: HTTPCheck) -> CheckResult:
        https_check = HTTPCheck(**{**check.__dict__, "scheme": "https", "port": check.port or 443})
        return self.http(https_check)

    def smtp(self, check: SMTPCheck) -> CheckResult:
        try:
            with smtplib.SMTP(check.host, check.port, timeout=self.timeout) as client:
                code, _ = client.noop()
                if code >= 400:
                    return CheckResult(False, f"SMTP NOOP failed with {code}")

                refused = client.sendmail(check.sender, [check.receiver], check.body)
                if refused:
                    return CheckResult(False, f"SMTP refused: {refused}")
        except Exception as exc:
            return CheckResult(False, f"smtp error: {exc}")

        return CheckResult(True, "SMTP send/receive step accepted")

    def pop3(self, check: POP3Check) -> CheckResult:
        client_cls = poplib.POP3_SSL if check.use_ssl else poplib.POP3
        try:
            client = client_cls(check.host, check.port, timeout=self.timeout)
        except Exception as exc:
            return CheckResult(False, f"pop3 connect error: {exc}")

        try:
            client.user(check.username)
            client.pass_(check.password)

            for cmd in check.commands:
                raw = client._shortcmd(cmd.command)  # low-level command to validate responses
                text = raw.decode("utf-8", errors="replace") if isinstance(raw, (bytes, bytearray)) else str(raw)
                if cmd.use_regex:
                    if not re.search(cmd.expected, text, re.IGNORECASE | re.MULTILINE):
                        return CheckResult(False, f"command {cmd.command} did not match regex")
                else:
                    if cmd.expected not in text:
                        return CheckResult(False, f"command {cmd.command} did not contain expected text")
        except Exception as exc:
            return CheckResult(False, f"pop3 auth/command error: {exc}")
        finally:
            try:
                client.quit()
            except Exception:
                client.close()

        return CheckResult(True, "POP3 login and commands passed")

    def ftp(self, check: FTPCheck) -> CheckResult:
        try:
            with ftplib.FTP() as ftp:
                ftp.connect(check.host, check.port, timeout=self.timeout)
                ftp.login(check.username, check.password)

                for expectation in check.files:
                    buffer = io.BytesIO()
                    ftp.retrbinary(f"RETR {expectation.name}", buffer.write)
                    data = buffer.getvalue()

                    if expectation.sha256:
                        digest = hashlib.sha256(data).hexdigest()
                        if digest != expectation.sha256.lower():
                            return CheckResult(False, f"{expectation.name} sha256 mismatch")

                    if expectation.regex:
                        text = data.decode("utf-8", errors="replace")
                        if not re.search(expectation.regex, text, re.IGNORECASE | re.MULTILINE | re.DOTALL):
                            return CheckResult(False, f"{expectation.name} content regex mismatch")
        except Exception as exc:
            return CheckResult(False, f"ftp error: {exc}")

        return CheckResult(True, "FTP authentication and file checks passed")

    def dns(self, check: DNSCheck) -> CheckResult:
        for record in check.records:
            kind = record.kind.upper()
            if kind == "A":
                try:
                    answers = {
                        addr[4][0]
                        for addr in socket.getaddrinfo(record.domain, None, proto=socket.IPPROTO_TCP)
                    }
                except Exception as exc:
                    return CheckResult(False, f"dns A lookup failed: {exc}")

                if not any(answer in answers for answer in record.answers):
                    return CheckResult(False, f"A record mismatch for {record.domain}")

            elif kind == "MX":
                try:
                    import dns.resolver
                except ImportError:
                    return CheckResult(False, "dnspython is required for MX lookups")

                resolver = dns.resolver.Resolver(configure=True)
                resolver.nameservers = [check.server]
                resolver.port = check.port
                resolver.timeout = self.timeout
                resolver.lifetime = self.timeout

                try:
                    mx_answers = resolver.resolve(record.domain, "MX")
                except Exception as exc:
                    return CheckResult(False, f"dns MX lookup failed: {exc}")

                normalized = {str(r.exchange).rstrip(".") for r in mx_answers}
                if not any(answer in normalized for answer in record.answers):
                    return CheckResult(False, f"MX record mismatch for {record.domain}")
            else:
                return CheckResult(False, f"unsupported DNS record kind: {record.kind}")

        return CheckResult(True, "DNS resolution passed")


# Example definitions extracted from sys.conf for later reuse in a web app.
HTTP_EXAMPLE = HTTPCheck(
    host="10.20.x.2",
    port=80,
    path="/joomla",
    expected_content="easy to get started creating your website",
    use_regex=True,
)

HTTPS_EXAMPLE = HTTPCheck(
    host="10.20.x.2",
    port=8006,
    path="/",
    scheme="https",
)

SMTP_EXAMPLE = SMTPCheck(
    host="10.20.x.2",
    port=25,
    sender="hello@scoring.engine",
    receiver="tuck@sherwood.lan",
    body="howdy, friar! he's about to have an outlaw for an inlaw!",
)

POP3_EXAMPLE = POP3Check(
    host="10.20.x.2",
    port=110,
    username="ad_user",
    password="Password1!",
    use_ssl=False,
    commands=[
        POP3Command(command="STAT", expected="+OK", use_regex=False),
    ],
)

FTP_EXAMPLE = FTPCheck(
    host="10.20.x.2",
    port=55,
    username="anonymous",
    password="anonymous@",
    files=[
        FTPFileExpectation(
            name="memo.txt",
            sha256="9d8453505bdc6f269678e16b3e56c2a2948a41f2c792617cc9611ed363c95b63",
        ),
        FTPFileExpectation(
            name="workfiles.txt",
            regex="work.*work",
        ),
    ],
)

DNS_EXAMPLE = DNSCheck(
    server="10.20.x.2",
    port=4000,
    records=[
        DNSRecordExpectation(
            kind="A",
            domain="townsquare.sherwood.lan",
            answers=["192.168.1.4"],
        ),
        DNSRecordExpectation(
            kind="MX",
            domain="sherwood.lan",
            answers=["192.168.1.5", "10.20.1.5"],
        ),
    ],
)

