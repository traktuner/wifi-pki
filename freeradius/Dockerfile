FROM freeradius/freeradius-server:3.2.11-alpine@sha256:0de028f73b233b64870d2aeaa0da04727778919c75ec430a560df340b724e3ad

ENV PATH="/opt/sbin:${PATH}"

# Site configuration is part of the image; only the client list (shared secret, authenticator addresses),
# the server certificate and the trust directory (CA chain, CRL) are supplied at run time.
COPY eap /opt/etc/raddb/mods-enabled/eap
COPY default /opt/etc/raddb/sites-enabled/default
COPY check-eap-tls /opt/etc/raddb/sites-enabled/check-eap-tls
COPY wifi-audit /opt/etc/raddb/mods-enabled/wifi-audit
COPY healthcheck.sh /usr/local/bin/wifi-radius-healthcheck
COPY refresh-crl.sh /usr/local/bin/wifi-radius-refresh-crl

# The upstream image is root-owned mode 0640 and defaults to root. RADIUS uses
# unprivileged UDP ports here, so make the immutable runtime configuration
# readable by a fixed numeric identity and drop root permanently.
RUN apk add --no-cache openssl curl \
    && chown -R 101:101 /opt/etc/raddb /opt/var \
    && rm -f \
      /opt/etc/raddb/sites-enabled/inner-tunnel \
      /opt/etc/raddb/mods-enabled/chap \
      /opt/etc/raddb/mods-enabled/digest \
      /opt/etc/raddb/mods-enabled/mschap \
      /opt/etc/raddb/mods-enabled/ntlm_auth \
      /opt/etc/raddb/mods-enabled/pap \
      /opt/etc/raddb/mods-enabled/passwd \
    && find /opt/etc/raddb -type d -exec chmod 0750 {} + \
    && find /opt/etc/raddb -type f -exec chmod 0440 {} + \
    && chmod 0555 /usr/local/bin/wifi-radius-healthcheck /usr/local/bin/wifi-radius-refresh-crl

USER 101:101
