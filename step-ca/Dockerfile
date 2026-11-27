FROM smallstep/step-ca:0.30.2@sha256:e9e8fa3262bf37b130962ffddbf6a64ac188f0bbb80959cf3ddc04c6bf294c3d

# Upstream applies cap_net_bind_service to step-ca. This stack listens on 9000,
# and a file capability cannot be executed together with no-new-privileges and
# cap_drop: ALL. A plain copy preserves the binary but drops the unneeded xattr.
USER root
RUN cp /usr/local/bin/step-ca /usr/local/bin/step-ca.unprivileged \
    && mv -f /usr/local/bin/step-ca.unprivileged /usr/local/bin/step-ca \
    && chmod 0755 /usr/local/bin/step-ca

USER 1000:1000
