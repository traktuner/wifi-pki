#!/bin/sh
# Fetch the CRL of the issuing CA, validate it and install it atomically into the trust directory that FreeRADIUS
# reads (ca_path, reloaded within 300 seconds). With "loop" it repeats every CRL_INTERVAL_SECONDS (default 3600).
#
# Environment:
#   CRL_URL                 CRL download address (required)
#   CA_BUNDLE               CA file that verifies the HTTPS connection (required)
#   TRUST_DIR               directory with intermediate_ca.crt, written to (default /opt/etc/raddb/certs/oncloud-trust)
#   ISSUER_CN               expected CN of the CRL issuer (required)
#   CRL_MIN_LIFETIME_SECONDS  nextUpdate minus lastUpdate must be at least this (default 157676400, five years)
#   CRL_MAX_AGE_SECONDS     warn when lastUpdate is older (default 14400)
#   CRL_ALERT_SECONDS       warn when nextUpdate is closer (default 2592000)
set -eu
umask 022
trust="${TRUST_DIR:-/opt/etc/raddb/certs/oncloud-trust}"
: "${CRL_URL:?}" "${CA_BUNDLE:?}" "${ISSUER_CN:?}"
min_life="${CRL_MIN_LIFETIME_SECONDS:-157676400}"
max_age="${CRL_MAX_AGE_SECONDS:-14400}"
alert="${CRL_ALERT_SECONDS:-2592000}"

refresh() {
    tmp="$(mktemp "$trust/.wifi.crl.XXXXXX")"
    trap 'rm -f "$tmp" "$tmp.pem"' EXIT
    curl --fail --silent --show-error --max-time 20 --cacert "$CA_BUNDLE" "$CRL_URL" -o "$tmp"
    if ! openssl crl -inform DER -in "$tmp" -out "$tmp.pem" 2>/dev/null; then
        openssl crl -inform PEM -in "$tmp" -out "$tmp.pem"
    fi
    openssl crl -in "$tmp.pem" -noout -verify -CAfile "$trust/intermediate_ca.crt" >/dev/null
    case "$(openssl crl -in "$tmp.pem" -noout -issuer)" in
        *"$ISSUER_CN"*) ;;
        *) echo "ERROR: CRL issuer mismatch" >&2; return 1 ;;
    esac
    last="$(openssl crl -in "$tmp.pem" -noout -lastupdate | cut -d= -f2-)"
    next="$(openssl crl -in "$tmp.pem" -noout -nextupdate | cut -d= -f2-)"
    last_epoch="$(date -u -D '%b %d %H:%M:%S %Y' -d "${last% GMT}" +%s)"
    next_epoch="$(date -u -D '%b %d %H:%M:%S %Y' -d "${next% GMT}" +%s)"
    now="$(date -u +%s)"
    [ "$last_epoch" -le $((now + 300)) ] || { echo "ERROR: CRL lastUpdate is in the future" >&2; return 1; }
    [ "$next_epoch" -gt "$now" ] || { echo "ERROR: CRL is expired" >&2; return 1; }
    [ $((next_epoch - last_epoch)) -ge "$min_life" ] || { echo "ERROR: CRL lifetime is shorter than the identity horizon" >&2; return 1; }
    [ "$last_epoch" -gt $((now - max_age)) ] || echo "WARNING: CRL source is older than the expected hourly generation window" >&2
    [ "$next_epoch" -gt $((now + alert)) ] || echo "WARNING: CRL expires within the alert window" >&2
    mv -f "$tmp.pem" "$trust/wifi.crl"
    openssl rehash "$trust" >/dev/null
    rm -f "$tmp"
    trap - EXIT
    echo "CRL validated and installed (lastUpdate ${last}, nextUpdate ${next})"
}

if [ "${1:-}" = "loop" ]; then
    while true; do
        refresh || echo "ERROR: CRL refresh failed, the installed CRL stays" >&2
        sleep "${CRL_INTERVAL_SECONDS:-3600}"
    done
else
    refresh
fi
