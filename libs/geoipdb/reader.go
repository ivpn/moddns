// Package geoipdb serves ASN lookups from a MaxMind database file and reopens
// the file when it is replaced on disk, so a refreshed GeoLite2 build is used
// without restarting the process.
package geoipdb

import (
	"context"
	"errors"
	"fmt"
	"math"
	"net"
	"os"
	"sync"
	"sync/atomic"
	"time"

	"github.com/oschwald/geoip2-golang"
	"github.com/rs/zerolog/log"
)

// DefaultReloadInterval is how often Watch checks the file when the caller
// passes a non-positive interval.
const DefaultReloadInterval = 15 * time.Minute

// Stats describes the database currently being served.
type Stats struct {
	BuildTime time.Time // build_epoch recorded in the database metadata
	LoadedAt  time.Time // when the current file was opened
	Reloads   uint64    // successful swaps since Open
	Failures  uint64    // replacements rejected since Open
	LastError string    // most recent reload error; empty after a success
}

// fileSig is what Reload compares to decide whether the file was replaced.
// geoipupdate writes a temporary file and renames it over the target, which
// always changes the modification time.
type fileSig struct {
	size    int64
	modTime time.Time
}

func (s fileSig) equal(o fileSig) bool {
	return s.size == o.size && s.modTime.Equal(o.modTime)
}

// Reader is safe for concurrent use and lock-free on the lookup path: the
// current database is an atomic pointer, and a swap simply publishes a new
// one. The previous database is never closed explicitly; lookups still
// holding it finish and the garbage collector reclaims it.
//
// The file is read into memory rather than mmapped. An mmap follows whatever
// a writer later does to the same inode, so an in-place overwrite would
// corrupt the database mid-lookup; a private copy is immune to that, and
// GeoLite2-ASN is small enough (~11 MB) for the copy to be free. It also
// means dropping a database releases nothing but heap.
type Reader struct {
	path string
	cur  atomic.Pointer[geoip2.Reader]

	mu    sync.Mutex // guards sig and stats; serialises Reload callers
	sig   fileSig
	stats Stats
}

// Open opens an ASN-capable database. A missing, unreadable, corrupt or
// wrong-edition file is an error.
func Open(path string) (*Reader, error) {
	if path == "" {
		return nil, errors.New("geoip database path is required")
	}
	db, sig, err := openFile(path)
	if err != nil {
		return nil, err
	}
	r := &Reader{
		path:  path,
		sig:   sig,
		stats: Stats{BuildTime: buildTime(db), LoadedAt: time.Now()},
	}
	r.cur.Store(db)
	return r, nil
}

func openFile(path string) (*geoip2.Reader, fileSig, error) {
	info, err := os.Stat(path)
	if err != nil {
		return nil, fileSig{}, err
	}
	if !info.Mode().IsRegular() {
		return nil, fileSig{}, fmt.Errorf("%s is not a regular file", path)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fileSig{}, err
	}
	db, err := geoip2.FromBytes(data)
	if err != nil {
		return nil, fileSig{}, fmt.Errorf("open %s: %w", path, err)
	}
	// geoip2 reports an edition/method mismatch only at lookup time, so probe
	// once here rather than on the first real query.
	if _, err := db.ASN(net.IPv4(192, 0, 2, 1)); err != nil {
		return nil, fileSig{}, fmt.Errorf("%s does not support ASN lookups: %w", path, err)
	}
	return db, fileSig{size: info.Size(), modTime: info.ModTime()}, nil
}

func buildTime(db *geoip2.Reader) time.Time {
	epoch := db.Metadata().BuildEpoch
	if epoch > math.MaxInt64 {
		epoch = math.MaxInt64
	}
	return time.Unix(int64(epoch), 0).UTC()
}

// Path returns the file the reader watches.
func (r *Reader) Path() string {
	return r.path
}

// ASN looks up ip in the database currently loaded. An address outside the
// database yields an empty record and no error.
func (r *Reader) ASN(ip net.IP) (*geoip2.ASN, error) {
	db := r.cur.Load()
	if db == nil {
		return nil, errors.New("geoip database is closed")
	}
	return db.ASN(ip)
}

// Stats returns a snapshot of the reader state.
func (r *Reader) Stats() Stats {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.stats
}

// Reload reopens the file if its size or modification time changed since it
// was last loaded. It reports whether a new database is now being served. A
// replacement that cannot be opened or is not an ASN database is rejected and
// the previous database keeps serving.
func (r *Reader) Reload() (bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.cur.Load() == nil {
		return false, errors.New("geoip database is closed")
	}

	info, err := os.Stat(r.path)
	if err != nil {
		return false, r.failLocked(err)
	}
	if r.sig.equal(fileSig{size: info.Size(), modTime: info.ModTime()}) {
		return false, nil
	}

	db, sig, err := openFile(r.path)
	if err != nil {
		return false, r.failLocked(err)
	}

	r.cur.Store(db)
	r.sig = sig
	r.stats.BuildTime = buildTime(db)
	r.stats.LoadedAt = time.Now()
	r.stats.Reloads++
	r.stats.LastError = ""

	log.Info().Str("path", r.path).Time("build_time", r.stats.BuildTime).Msg("GeoIP database reloaded")
	return true, nil
}

// failLocked records a rejected replacement; r.mu must be held.
func (r *Reader) failLocked(err error) error {
	r.stats.Failures++
	r.stats.LastError = err.Error()
	log.Error().Err(err).Str("path", r.path).Msg("GeoIP database reload rejected; previous database kept")
	return err
}

// Watch calls Reload every interval until ctx is cancelled.
func (r *Reader) Watch(ctx context.Context, every time.Duration) {
	if r == nil {
		return
	}
	if every <= 0 {
		every = DefaultReloadInterval
	}
	ticker := time.NewTicker(every)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			_, _ = r.Reload()
		}
	}
}

// Close stops serving lookups; later calls to ASN return an error. The
// in-memory database holds no file mapping, so there is nothing else to
// release, and it is not closed explicitly because a lookup may still be
// using it.
func (r *Reader) Close() error {
	r.cur.Store(nil)
	return nil
}
