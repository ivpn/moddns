# NOTE: The image should be built from the root of the project (see Makefile)
FROM golang:1.25.8 AS build

WORKDIR /app

# Copy golang dependency manifests
COPY proxy/go.mod .
COPY proxy/go.sum .

# Copy the DNS libs
COPY ../libs/ /libs

# Cache the downloaded dependency in the layer
RUN go mod download
RUN go mod verify

# Copy the source code
COPY proxy/ .

RUN CGO_ENABLED=0 go build -o /app/dnsproxy .

FROM scratch AS run

COPY --from=build /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=build /app/dnsproxy /bin/dnsproxy
ENTRYPOINT ["/bin/dnsproxy"]
