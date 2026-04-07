### How to test modDNS service via browser

#### Firefox

1. Add locally generated root certificate to the browser

- Search for Certificates --> View Certificates --> Authorities --> Import
- location of root cert in repo: `certs/mkcert_development_CA_307611231582065277882115426409270736451.crt`

2. Set DNS settings for your browser

- Search for DNS --> Enable DNS over HTTPS using --> Check Max Protection
- Paste your custom endpoint into `custom` provider field: `https://ivpndns.com:443/dns-query/7geckax1e5`

### Test DNS over HTTPS

```
dig +https=/dns-query/ju8eamnqfn @ivpndns.com google.com
```

OR

```
q -S jinghuazhijia.com A @https://ivpndns.com:443/dns-query/7geckax1e5 -v
```

OR

```
maciek@maciek-ThinkPad-T14s-Gen-4:~$ dog example.com --https @https://cloudflare-dns.com/dns-query
A example.com. 23h12m06s   93.184.216.34
```

In this case real DoH server can be pointed to (I'm using cloudflare DoH endpoint here):

```
maciek@maciek-ThinkPad-T14s-Gen-4:~$ dog example.com --https @https://ivpndns.com/dns-query
A example.com. 59m49s   93.184.216.34
```

In case unbound is configured as upstream (not sure if queries from Unbound up are encrypted, Unbound also supports /dns-query endpoint):

```
dog wp.pl --https @https://ivpndns.com/dns-query
```

JSON output:

```
dog wp.pl -J --time --https @https://ivpndns.com/dns-query | jq
```

### DNS over TLS

```
maciek@maciek-ThinkPad-T14s-Gen-4:~$ kdig -d @ivpndns.com:853 +tls-ca wp.pl
;; DEBUG: Querying for owner(wp.pl.), class(1), type(1), server(ivpndns.com), port(853), protocol(TCP)
;; DEBUG: TLS, imported 139 system certificates
;; DEBUG: TLS, received certificate hierarchy:
;; DEBUG:  #1, O=mkcert development certificate,OU=maciek@maciek-ThinkPad-T14s-Gen-4 (Maciek)
;; DEBUG:      SHA-256 PIN: 4PRWwapk+ZA3PVrskDGi6C2CHMNpT1laXFujVP/RKxg=
;; DEBUG: TLS, skipping certificate PIN check
;; DEBUG: TLS, The certificate is trusted.
;; TLS session (TLS1.3)-(ECDHE-X25519)-(RSA-PSS-RSAE-SHA256)-(AES-128-GCM)
;; ->>HEADER<<- opcode: QUERY; status: NOERROR; id: 18729
;; Flags: qr rd ra; QUERY: 1; ANSWER: 1; AUTHORITY: 0; ADDITIONAL: 1

;; EDNS PSEUDOSECTION:
;; Version: 0; flags: ; UDP size: 4096 B; ext-rcode: NOERROR
;; PADDING: 90 B

;; QUESTION SECTION:
;; wp.pl.              		IN	A

;; ANSWER SECTION:
wp.pl.              	297	IN	A	212.77.98.9

;; Received 149 B
;; Time 2024-04-09 10:49:47 CEST
;; From 127.0.0.1@853(TCP) in 43.6 ms
```

```
kdig -d +tls-ca @uakp9sroa8.dns.staging.ivpndns.net wp.pl
```

```
maciek@maciek-ThinkPad-T14s-Gen-4:~/git/q$ ./q -S cryptome.org A @https://dns.staging.ivpndns.net/dns-query/7geckax1e5 -vvvv
DEBU[0000] Name: cryptome.org                           
DEBU[0000] RR types: [A]                                
DEBU[0000] Server(s): [https://dns.staging.ivpndns.net/dns-query/7geckax1e5] 
DEBU[0000] Using server https://dns.staging.ivpndns.net:443/dns-query/7geckax1e5 with transport http 
DEBU[0000] Using HTTP(s) transport: https://dns.staging.ivpndns.net:443/dns-query/7geckax1e5 
DEBU[0000] [http] sending GET request to https://dns.staging.ivpndns.net:443/dns-query/7geckax1e5?dns=YcABAAABAAAAAAAACGNyeXB0b21lA29yZwAAAQAB 
FATA[0000] unpacking DNS response from https://dns.staging.ivpndns.net:443/dns-query/7geckax1e5?dns=YcABAAABAAAAAAAACGNyeXB0b21lA29yZwAAAQAB: dns: overflow unpacking uint16
```

Q examples:
```
./q wp.pl A @tls://ju8y5re4zc.dns.staging.ivpndns.net -v


./q wp.pl A @quic://ju8y5re4zc.dns.staging.ivpndns.net -v
```

Local server:
```
./q wp.pl A @tls://test-3mdq3851b9.ivpndns.com -v
```

### Location Subdomain Testing (DoT/DoQ)

Each proxy node can serve both the anycast domain and a location-specific domain.
Replace `PROFILEID` with a real profile ID and `STAGING_IP` with the staging node IP.

DoT via location subdomain (vm1):
```
kdig @STAGING_IP +tls-sni=PROFILEID.vm1.dns.staging.ivpndns.net \
  +tls-ca +tls example.com A -p 853
```

DoT via anycast (same node, for comparison):
```
kdig @STAGING_IP +tls-sni=PROFILEID.dns.staging.ivpndns.net \
  +tls-ca +tls example.com A -p 853
```

DoQ via location subdomain:
```
kdig @STAGING_IP +quic-sni=PROFILEID.vm1.dns.staging.ivpndns.net \
  +tls-ca +quic example.com A -p 853
```

Verify which cert is served (check SAN/CN):
```
kdig @STAGING_IP +tls-sni=PROFILEID.vm1.dns.staging.ivpndns.net \
  +tls-ca +tls +tls-debug example.com A -p 853 2>&1 | grep -i "certificate"
```

If the location cert isn't yet trusted (e.g. self-signed for testing), use `+tls-no-ca` instead of `+tls-ca`.

Production locations: `tor1.dns.moddns.net` (dfn1), `ams1.dns.moddns.net` (dfn3).

### DNS over QUIC

I found a DNS client which is capable of sending all mentioned types of DNS requests, not only DoQ: https://github.com/natesales/q

```
maciek@maciek-ThinkPad-T14s-Gen-4:~/git/coredns$ q -S wp.pl A @quic://123asd3.ivpndns.com
2024/04/22 16:39:56 failed to sufficiently increase send buffer size (was: 208 kiB, wanted: 2048 kiB, got: 416 kiB). See https://github.com/quic-go/quic-go/wiki/UDP-Buffer-Sizes for details.
wp.pl. 2m51s A 212.77.98.9
Stats:
Received 44 B from 123asd3.ivpndns.com in 27.3ms (16:39:56 04-22-2024 CEST)
Opcode: QUERY Status: NOERROR ID 0: Flags: qr aa rd ra (1 Q 1 A 0 N 0 E)
```

### Test redis internals

```
docker exec -it redis bash
redis-cli --user api --pass apipass
```

### Postman REST API collection

In order to get Postman collection for DNS REST API, swagger docs can be imported. More info: `https://learning.postman.com/docs/getting-started/importing-and-exporting/importing-from-swagger/#import-a-swagger-api`


### DNSSEC


```
dig +https=/dns-query/ju8eamnqfn @ivpndns.com ietf.org +dnssec +multiline
```


Improperly configured DNSSEC domain:
```
dig +https=/dns-query/ju8eamnqfn @ivpndns.com dnssec-failed.org +dnssec +multiline
```

### Passkeys

Chrome has built-in, virtual authenticator feature: https://developer.chrome.com/docs/devtools/webauthn/
