module github.com/ivpn/dns/proxy

go 1.23.2

require (
	github.com/AdguardTeam/dnsproxy v0.70.0
	github.com/AdguardTeam/golibs v0.23.1
	github.com/getsentry/sentry-go/zerolog v0.31.1
	github.com/ivpn/dns/libs v0.0.0
	github.com/miekg/dns v1.1.58
	github.com/oschwald/geoip2-golang v1.13.0
	github.com/quic-go/quic-go v0.42.0
	github.com/redis/go-redis/v9 v9.7.3
	github.com/rs/zerolog v1.34.0
	github.com/stretchr/testify v1.10.0
)

replace github.com/ivpn/dns/libs => ../libs

require (
	github.com/buger/jsonparser v1.1.1 // indirect
	github.com/cenkalti/backoff/v4 v4.3.0 // indirect
	github.com/davecgh/go-spew v1.1.1 // indirect
	github.com/golang-migrate/migrate/v4 v4.18.2 // indirect
	github.com/golang/snappy v1.0.0 // indirect
	github.com/hashicorp/errwrap v1.1.0 // indirect
	github.com/hashicorp/go-multierror v1.1.1 // indirect
	github.com/klauspost/compress v1.18.0 // indirect
	github.com/montanaflynn/stats v0.7.1 // indirect
	github.com/oschwald/maxminddb-golang v1.13.0 // indirect
	github.com/pmezard/go-difflib v1.0.0 // indirect
	github.com/stretchr/objx v0.5.2 // indirect
	github.com/xdg-go/pbkdf2 v1.0.0 // indirect
	github.com/xdg-go/scram v1.1.2 // indirect
	github.com/xdg-go/stringprep v1.0.4 // indirect
	github.com/youmark/pkcs8 v0.0.0-20240726163527-a2c0da244d78 // indirect
	go.uber.org/atomic v1.11.0 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
)

require (
	github.com/aead/chacha20 v0.0.0-20180709150244-8b13a72661da // indirect
	github.com/aead/poly1305 v0.0.0-20180717145839-3fee0db0b635 // indirect
	github.com/allegro/bigcache/v3 v3.1.0
	github.com/ameshkov/dnscrypt/v2 v2.2.7 // indirect
	github.com/ameshkov/dnsstamps v1.0.3 // indirect
	github.com/beefsack/go-rate v0.0.0-20220214233405-116f4ca011a0 // indirect
	github.com/bluele/gcache v0.0.2 // indirect
	github.com/cespare/xxhash/v2 v2.3.0 // indirect
	github.com/dgryski/go-rendezvous v0.0.0-20200823014737-9f7001d12a5f // indirect
	github.com/getsentry/sentry-go v0.31.1
	github.com/go-task/slim-sprig v0.0.0-20230315185526-52ccab3ef572 // indirect
	github.com/google/pprof v0.0.0-20240130152714-0ed6a68c8d9e // indirect
	github.com/mattn/go-colorable v0.1.14 // indirect
	github.com/mattn/go-isatty v0.0.20 // indirect
	github.com/onsi/ginkgo/v2 v2.15.0 // indirect
	github.com/patrickmn/go-cache v2.1.0+incompatible // indirect
	github.com/quic-go/qpack v0.4.0 // indirect
	go.mongodb.org/mongo-driver v1.17.3
	go.uber.org/mock v0.4.0 // indirect
	golang.org/x/crypto v0.36.0 // indirect
	golang.org/x/exp v0.0.0-20240409090435-93d18d7e34b8 // indirect
	golang.org/x/mod v0.21.0 // indirect
	golang.org/x/net v0.35.0 // indirect
	golang.org/x/sync v0.12.0
	golang.org/x/sys v0.31.0 // indirect
	golang.org/x/text v0.23.0 // indirect
	golang.org/x/tools v0.24.0 // indirect
	gonum.org/v1/gonum v0.14.0 // indirect
)
