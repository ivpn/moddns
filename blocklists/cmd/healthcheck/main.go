// Command healthcheck probes the service's liveness endpoint for the Docker
// HEALTHCHECK: the image is built FROM scratch, so a shell one-liner is not
// available. Liveness (not readiness) is deliberate — restarting the
// container cannot fix an unreachable Mongo/Redis, only a wedged process.
package main

import (
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"
)

const defaultPort = "9153"

func main() {
	port := strings.TrimLeft(os.Getenv("METRICS_PORT"), ":")
	if port == "" {
		port = defaultPort
	}

	client := &http.Client{Timeout: 2 * time.Second}
	resp, err := client.Get(fmt.Sprintf("http://localhost:%s/health/live", port)) //nolint:gosec // G704 - URL from local env var, not user input
	if err != nil || resp.StatusCode != http.StatusOK {
		os.Exit(1)
	}
}
