### Development Certificates

This directory contains certificates necessary for local development and testing.


1. `private_key.pem` and `certificate.pem` are used in API unit tests and integration tests (mobileconfig generation).
2. `moddns.dev+4.pem` and `moddns.dev+4-key.pem` are the TLS server cert/key (SANs: `moddns.dev`, `*.moddns.dev`, `localhost`, `127.0.0.1`, `::1`) used for local development and in integration tests. The proxy serves them for DoH/DoT/DoQ on `moddns.dev`.
3. `moddns_dev_development_CA.crt` is the root CA that signed the cert above. It is trusted by the integration test client (both locally via `tests/Dockerfile` and in the GitHub workflow) so `https://moddns.dev` validates.

#### Regenerating on expiry

The CA private key is intentionally **not** committed, so a new CA + leaf are minted together:

```sh
# 1. New development CA (keep CA key out of the repo)
openssl req -x509 -new -nodes -newkey rsa:4096 -sha256 -days 3650 \
  -keyout /tmp/moddns_dev_CA-key.pem -out moddns_dev_development_CA.crt \
  -subj "/O=modDNS development CA/CN=modDNS development CA"

# 2. Leaf key + CSR
openssl req -new -nodes -newkey rsa:2048 -sha256 \
  -keyout moddns.dev+4-key.pem -out /tmp/leaf.csr \
  -subj "/O=modDNS development certificate"

# 3. Sign the leaf with the SANs (server auth)
printf 'basicConstraints=CA:FALSE\nkeyUsage=critical,digitalSignature,keyEncipherment\nextendedKeyUsage=serverAuth\nsubjectAltName=DNS:moddns.dev,DNS:*.moddns.dev,DNS:localhost,IP:127.0.0.1,IP:0:0:0:0:0:0:0:1\n' > /tmp/leaf-ext.cnf
openssl x509 -req -in /tmp/leaf.csr -CA moddns_dev_development_CA.crt \
  -CAkey /tmp/moddns_dev_CA-key.pem -CAcreateserial -out moddns.dev+4.pem \
  -days 3650 -sha256 -extfile /tmp/leaf-ext.cnf
```

If the CA filename changes, update `tests/Dockerfile` and `.github/workflows/integration_tests.yml`.
