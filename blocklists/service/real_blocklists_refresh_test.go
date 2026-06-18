package service

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

const (
	fixtureSampleN  = 10000             // domains kept per fixture
	fixtureMaxBytes = 256 * 1024 * 1024 // safety cap on a single download
	fixtureScanLine = 1024 * 1024       // generous per-line cap for the refresher
)

// TestRefreshFixtures downloads each upstream list, keeps its header block, and
// writes a random 10k sample of the body to testdata/real/. It is a manual
// fixture generator, NOT a CI test: it is skipped unless REFRESH_FIXTURES is
// set, and it reaches the network. Each run reseeds, so the committed sample
// rotates over time (sampling the whole list, not just the head).
//
//	REFRESH_FIXTURES=1 go test ./service -run TestRefreshFixtures -count=1
func TestRefreshFixtures(t *testing.T) {
	if os.Getenv("REFRESH_FIXTURES") == "" {
		t.Skip("set REFRESH_FIXTURES=1 to download and regenerate testdata/real fixtures")
	}
	if err := os.MkdirAll(fixtureDir, 0o755); err != nil {
		t.Fatal(err)
	}

	rng := rand.New(rand.NewSource(time.Now().UnixNano()))
	client := &http.Client{Timeout: 90 * time.Second}

	for _, fx := range realFixtures {
		fx := fx
		t.Run(fx.file, func(t *testing.T) {
			body, err := fetchURL(client, fx.url)
			if err != nil {
				t.Fatalf("download %s: %v", fx.url, err)
			}

			header, data := splitHeaderAndData(body)
			rng.Shuffle(len(data), func(i, j int) { data[i], data[j] = data[j], data[i] })
			if len(data) > fixtureSampleN {
				data = data[:fixtureSampleN]
			}

			var buf bytes.Buffer
			for _, h := range header {
				buf.WriteString(h)
				buf.WriteByte('\n')
			}
			for _, d := range data {
				buf.WriteString(d)
				buf.WriteByte('\n')
			}

			path := filepath.Join(fixtureDir, fx.file)
			if err := os.WriteFile(path, buf.Bytes(), 0o644); err != nil {
				t.Fatal(err)
			}
			t.Logf("%s: %d header lines + %d sampled domains -> %s", fx.extractor, len(header), len(data), path)
		})
	}
}

// fetchURL GETs url with a browser-like User-Agent (some CDNs reject the
// default Go agent) and returns the body, capped at fixtureMaxBytes.
func fetchURL(client *http.Client, url string) ([]byte, error) {
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (compatible; moddns-fixture-refresher/1.0)")
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("bad status: %s", resp.Status)
	}
	return io.ReadAll(io.LimitReader(resp.Body, fixtureMaxBytes))
}

// splitHeaderAndData returns the leading contiguous comment/blank block verbatim
// (so strict extractors still find their Last-Modified/entries headers) and the
// remaining non-comment, non-blank data lines (interspersed comments dropped).
func splitHeaderAndData(body []byte) (header, data []string) {
	sc := bufio.NewScanner(bytes.NewReader(body))
	sc.Buffer(make([]byte, 0, 64*1024), fixtureScanLine)
	inHeader := true
	for sc.Scan() {
		line := sc.Text()
		trimmed := strings.TrimSpace(line)
		isCommentOrBlank := trimmed == "" || strings.HasPrefix(trimmed, "#") || strings.HasPrefix(trimmed, "!")
		if inHeader {
			if isCommentOrBlank {
				header = append(header, line)
				continue
			}
			inHeader = false
		}
		if isCommentOrBlank {
			continue
		}
		data = append(data, line)
	}
	return header, data
}
