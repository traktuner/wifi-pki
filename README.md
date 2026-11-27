# wifi-pki

Building blocks of a small EAP-TLS Wi-Fi setup for Apple devices: one non-exportable client identity per device instead of a
shared Wi-Fi password.

| Directory | Image | Purpose |
|---|---|---|
| `enrollment/` | `ghcr.io/traktuner/wifi-enrollment` | Go web server: OIDC sign-in, SCEP challenge webhook for step-ca, signed `.mobileconfig` profile download, SQLite store |
| `freeradius/` | `ghcr.io/traktuner/wifi-freeradius` | FreeRADIUS 3.2 with EAP-TLS only, CRL check, device identity check; helper scripts for the CRL download and the health check |
| `step-ca/` | `ghcr.io/traktuner/wifi-step-ca` | smallstep `step-ca` without the file capability, so it runs with `cap_drop: ALL` and `no-new-privileges` (it also carries the `step` CLI) |

The repository holds code and generic configuration only. Addresses, host names, SSIDs, secrets, the client list of the RADIUS
server and the CA state are supplied at run time by the deployment (environment variables, mounted files). Nothing in here
names a real network.

## Identifiers that stay as they are

`urn:oncloud:wifi-device:<uuid>` (URI SAN of every issued device certificate, checked by `freeradius/check-eap-tls`) and the profile
payload identifiers `at.oncloud.wifi.*` are part of already issued certificates and installed profiles. Changing them would
invalidate those; they are identifiers, not addresses.

## Run time contract

- **enrollment:** configuration through environment variables (`config.go`), secrets through `*_FILE` paths, the state directory
  holds the SQLite file. The health check is `wifi-enrollment healthcheck http://127.0.0.1:8080/healthz`.
- **freeradius:** mounts `clients.conf`, `certs/oncloud` (`radius.crt`, `radius.key`) and `certs/oncloud-trust` (`root_ca.crt`,
  `intermediate_ca.crt`, `wifi.crl`). `wifi-radius-refresh-crl` keeps the CRL current (see the header of the script),
  `wifi-radius-healthcheck` is the container health check.
- **step-ca:** runs `/usr/local/bin/step-ca <ca.json> --password-file <file>`; configuration and keys come from volumes.

## Build

GitHub Actions (`.github/workflows/build.yml`) builds `linux/amd64` images on every push to `main`. The enrollment Dockerfile runs
`gofmt` and `go test` in the build stage.
