#!/bin/bash
set -e

PROXY_PORT=8053
PROXY_ADDR="127.0.0.1"

echo "Testing A query for google.com (has AAAA)..."
RESULT=$(dig @$PROXY_ADDR -p $PROXY_PORT google.com A +short)
echo "$RESULT" | grep -q ":" && echo "Success: Got AAAA record" || (echo "Fail: Expected AAAA record"; exit 1)

echo "Testing A query for v4.ident.me (no AAAA)..."
RESULT=$(dig @$PROXY_ADDR -p $PROXY_PORT v4.ident.me A +short)
echo "$RESULT" | grep -q "\." && echo "Success: Got A record" || (echo "Fail: Expected A record"; exit 1)

echo "Testing AAAA query for google.com..."
RESULT=$(dig @$PROXY_ADDR -p $PROXY_PORT google.com AAAA +short)
echo "$RESULT" | grep -q ":" && echo "Success: Got AAAA record" || (echo "Fail: Expected AAAA record"; exit 1)

echo "Testing MX query for google.com..."
RESULT=$(dig @$PROXY_ADDR -p $PROXY_PORT google.com MX +short)
echo "$RESULT" | grep -q "smtp.google.com" && echo "Success: Got MX record" || (echo "Fail: Expected MX record"; exit 1)

echo "Testing TCP A query for google.com..."
RESULT=$(dig +tcp @$PROXY_ADDR -p $PROXY_PORT google.com A +short)
echo "$RESULT" | grep -q ":" && echo "Success: Got AAAA record over TCP" || (echo "Fail: Expected AAAA record over TCP"; exit 1)

echo "All tests passed!"
