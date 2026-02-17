# NOTE: The image should be built from the root of the project (see Makefile)
FROM golang:1.25.4 AS build

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

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /app/dnsapi .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /app/healthcheck ./cmd/healthcheck

FROM scratch AS run

COPY --from=build /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=build /app/db/mongodb/migrations /db/mongodb/migrations
COPY --from=build /app/dnsapi /bin/dnsapi
COPY --from=build /app/healthcheck /bin/healthcheck
ENTRYPOINT ["/bin/dnsapi"]
