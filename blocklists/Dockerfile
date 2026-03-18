# NOTE: The image should be built from the root of the project (see Makefile)
FROM golang:1.25.8 AS build

WORKDIR /app

# Copy golang dependency manifests
COPY blocklists/go.mod .
COPY blocklists/go.sum .

# Copy the DNS libs
COPY ../libs/ /libs

# Cache the downloaded dependency in the layer
RUN go mod download
RUN go mod verify

# Copy the source code
COPY blocklists/ .

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /app/blocklists .

FROM scratch AS run

COPY --from=build /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=build /app/blocklists /bin/blocklists
ENTRYPOINT ["/bin/blocklists"]
