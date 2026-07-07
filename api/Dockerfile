# NOTE: The image should be built from the root of the project (see Makefile)
FROM golang:1.25.8 AS build

WORKDIR /app

# Copy golang dependency manifests
COPY api/go.mod .
COPY api/go.sum .

# Copy the DNS libs
COPY ../libs/ /libs

# Cache the downloaded dependency in the layer
RUN go mod download
RUN go mod verify

# Copy the source code
COPY api/ .

# APP_VERSION is the deployed image tag, supplied at docker build time
# (see ansible/build-and-push-images.yml: service_matrix.api.build_args).
# When unset, the version package's "dev" default is preserved.
ARG APP_VERSION=dev

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
    go build \
      -ldflags="-X github.com/ivpn/dns/api/internal/version.Version=${APP_VERSION}" \
      -o /app/dnsapi .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /app/healthcheck ./cmd/healthcheck

FROM scratch AS run

COPY --from=build /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=build /app/db/mongodb/migrations /db/mongodb/migrations
COPY --from=build /app/dnsapi /bin/dnsapi
COPY --from=build /app/healthcheck /bin/healthcheck
ENTRYPOINT ["/bin/dnsapi"]
