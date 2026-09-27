#!/bin/bash

# Functional mail check with curl (replaces the banner-only check).
#
# Usage:
#   mail_check.sh smtp <host> <port> <from> <to>
#       Sends a probe message over SMTP (no auth, local domain). Passes when
#       the server accepts it for delivery.
#   mail_check.sh imap <host> <port> <user> <password>
#       Logs in over IMAP (143, no TLS), searches INBOX, reads the newest
#       message and removes old scoreboard probes so the mailbox stays small.
#
# Errors go to stderr and tell apart "service down" from "wrong password".
# Everything must finish inside the scoreboard's 5 second limit, so each curl
# call only gets the time that is left of BUDGET_MS.

PROBE_SUBJECT="Scoreboard probe"
BUDGET_MS=4600
START_MS=$(date +%s%3N)

usage() {
    echo "Usage: $0 smtp <host> <port> <from> <to> | imap <host> <port> <user> <password>" >&2
    exit 1
}

fail() {
    echo "$*" >&2
    exit 1
}

[ $# -eq 5 ] || usage
MODE=$1 HOST=$2 PORT=$3
[ -n "$HOST" ] && [ -n "$PORT" ] || usage

TRACE=$(mktemp)
trap 'rm -f "$TRACE"' EXIT

# Seconds left of the budget, e.g. "2.350"
remaining() {
    local left=$((BUDGET_MS - ($(date +%s%3N) - START_MS)))
    [ $left -lt 200 ] && left=200
    printf '%d.%03d' $((left / 1000)) $((left % 1000))
}

# run_curl <args...>: runs curl with a verbose trace in $TRACE, sets RC and OUT
run_curl() {
    local left
    left=$(remaining)
    # For IMAP/SMTP curl's connect timeout also covers the login, so give it
    # the whole budget: Dovecot answers a wrong password only after ~2-4s.
    OUT=$(curl --silent --show-error --verbose --stderr "$TRACE" \
        --connect-timeout "$left" --max-time "$left" "$@")
    RC=$?
}

# Prints the last line the server sent, to include it in error messages
server_reply() {
    grep '^< ' "$TRACE" | tail -n 1 | sed 's/^< //' | tr -d '\r'
}

# Explains a failed curl call: service down vs. credentials vs. other
explain_failure() {
    local proto=$1 user=$2 greeting=$3
    if ! grep -q '^\* Connected to' "$TRACE"; then
        fail "SERVICIO CAIDO: no se pudo conectar a $proto $HOST:$PORT (curl $RC)"
    fi
    if ! grep -q "^< $greeting" "$TRACE"; then
        fail "SERVICIO NO RESPONDE: $HOST:$PORT acepta la conexion pero no envia el saludo $proto (curl $RC)"
    fi
    if [ "$proto" = IMAP ]; then
        if [ $RC -eq 67 ] || grep -qi 'AUTHENTICATIONFAILED\|authentication failed' "$TRACE"; then
            fail "CONTRASENA INCORRECTA: el servidor rechazo el login de $user en $HOST:$PORT"
        fi
        if [ $RC -eq 28 ] && grep -q '^> A[0-9]* \(AUTHENTICATE\|LOGIN\)' "$TRACE" &&
            ! grep -q '^< A[0-9]* OK .*\(Logged in\|authenticat\)' "$TRACE"; then
            fail "LOGIN SIN RESPUESTA (posible contrasena incorrecta): $HOST:$PORT no respondio al login de $user a tiempo; Dovecot retrasa las respuestas tras logins fallidos"
        fi
    fi
    fail "$proto $HOST:$PORT fallo (curl $RC): $(server_reply)"
}

smtp_check() {
    local from=$1 to=$2 token
    [ -n "$from" ] && [ -n "$to" ] || usage
    token="$(date +%s)-$$"

    run_curl --url "smtp://$HOST:$PORT" --mail-from "$from" --mail-rcpt "$to" --upload-file - < <(
        printf '%s\r\n' \
            "From: $from" \
            "To: $to" \
            "Subject: $PROBE_SUBJECT $token" \
            '' \
            "Mensaje de prueba del scoreboard ($token)."
    )
    if [ $RC -ne 0 ]; then
        if grep -q '^< 220' "$TRACE" && grep -q '^< [45][0-9][0-9]' "$TRACE"; then
            fail "SMTP RECHAZO EL CORREO de $from a $to en $HOST:$PORT: $(grep '^< [45][0-9][0-9]' "$TRACE" | tail -n 1 | sed 's/^< //' | tr -d '\r')"
        fi
        explain_failure SMTP "" 220
    fi
    echo "SMTP acepto el correo de prueba $token para $to"
}

imap_check() {
    local user=$1 pass=$2 uids newest probes
    [ -n "$user" ] && [ -n "$pass" ] ||
        fail "FALTAN CREDENCIALES: define MAIL_USER/MAIL_PASS para este participante"
    local base="imap://$HOST:$PORT/INBOX"

    # Login + SELECT INBOX + UID SEARCH ALL
    run_curl --user "$user:$pass" --url "$base" --request 'UID SEARCH ALL'
    [ $RC -eq 0 ] || explain_failure IMAP "$user" '\* OK'
    echo "$OUT" | grep -q '^\* SEARCH' ||
        fail "IMAP respuesta inesperada a SEARCH en $HOST:$PORT: $OUT"

    uids=$(echo "$OUT" | tr -d '\r' | sed -n 's/^\* SEARCH *//p')
    if [ -z "$uids" ]; then
        echo "IMAP login OK para $user, INBOX vacio"
        exit 0
    fi

    # Read the newest message (headers + body)
    newest=$(echo "$uids" | tr ' ' '\n' | sort -n | tail -n 1)
    run_curl --user "$user:$pass" --url "$base/;UID=$newest"
    [ $RC -eq 0 ] || explain_failure IMAP "$user" '\* OK'
    [ -n "$OUT" ] || fail "IMAP devolvio el mensaje UID $newest vacio para $user"
    echo "IMAP login OK para $user, leido UID $newest"

    # Best-effort cleanup of old probes; a failure here does not fail the check.
    run_curl --user "$user:$pass" --url "$base" --request "UID SEARCH SUBJECT \"$PROBE_SUBJECT\""
    probes=$(echo "$OUT" | tr -d '\r' | sed -n 's/^\* SEARCH *//p' | tr ' ' ',')
    if [ $RC -eq 0 ] && [ -n "$probes" ]; then
        run_curl --user "$user:$pass" --url "$base" --request "UID STORE $probes +FLAGS.SILENT \\Deleted"
        [ $RC -eq 0 ] && run_curl --user "$user:$pass" --url "$base" --request 'EXPUNGE'
    fi
    exit 0
}

case $MODE in
    smtp) smtp_check "$4" "$5" ;;
    imap) imap_check "$4" "$5" ;;
    *) usage ;;
esac
