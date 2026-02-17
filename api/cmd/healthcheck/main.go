package main

import (
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"
)

const defaultPort = "3000"

func main() {
	port := strings.TrimLeft(os.Getenv("API_PORT"), ":")
	if port == "" {
		port = defaultPort
	}

	client := &http.Client{Timeout: 2 * time.Second}
	resp, err := client.Get(fmt.Sprintf("http://localhost:%s/health/ready", port))
	if err != nil || resp.StatusCode != http.StatusOK {
		os.Exit(1)
	}
}
