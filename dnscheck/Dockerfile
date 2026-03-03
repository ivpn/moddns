# NOTE: The image should be built from the root of the project (see Makefile)
FROM golang:1.25.7 AS build

WORKDIR /app

# Copy golang dependency manifests
COPY dnscheck/go.mod .
COPY dnscheck/go.sum .

# Cache the downloaded dependency in the layer
RUN go mod download

# Copy the source code
COPY dnscheck/ .

RUN CGO_ENABLED=0 go build -o /app/dnscheck .

FROM scratch AS run

EXPOSE 3000

COPY --from=build /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=build /app/dnscheck /bin/dnscheck
ENTRYPOINT ["/bin/dnscheck"]
