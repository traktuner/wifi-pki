#!/bin/sh
# Healthy when the configuration is valid and the CRL is current: lastUpdate not older than CRL_MAX_AGE_SECONDS
# (default 4 hours, the CRL is regenerated hourly) and nextUpdate in the future.
set -eu
crl="${TRUST_DIR:-/opt/etc/raddb/certs/oncloud-trust}/wifi.crl"
max_age="${CRL_MAX_AGE_SECONDS:-14400}"
radiusd -XC >/dev/null 2>&1
last="$(openssl crl -in "$crl" -noout -lastupdate | cut -d= -f2-)"
last="${last% GMT}"
next="$(openssl crl -in "$crl" -noout -nextupdate | cut -d= -f2-)"
next="${next% GMT}"
now="$(date -u +%s)"
[ "$(date -u -D '%b %d %H:%M:%S %Y' -d "$last" +%s)" -gt $((now - max_age)) ]
[ "$(date -u -D '%b %d %H:%M:%S %Y' -d "$next" +%s)" -gt "$now" ]
