package settingscache

import "github.com/ivpn/dns/proxy/model"

// Fixed per-object overheads for the estimate: the entry with its slice
// headers and LRU node, a map header, and a key/value pair of string headers.
// The result is a retained-bytes trend, not heap accounting.
const (
	entryOverhead = 256
	mapOverhead   = 48
	kvOverhead    = 32
	stringHeader  = 16
)

// estimateBytes approximates the memory retained by one cached profile.
func estimateBytes(s *model.ProfileSettings) int64 {
	if s == nil {
		return entryOverhead
	}
	n := int64(entryOverhead)
	for _, m := range []map[string]string{s.Privacy, s.Logs, s.DNSSEC, s.RebindingProtection, s.Advanced, s.Statistics} {
		n += mapBytes(m)
	}
	for _, v := range s.Blocklists {
		n += stringHeader + int64(len(v))
	}
	for _, v := range s.Services {
		n += stringHeader + int64(len(v))
	}
	for _, r := range s.CustomRules {
		n += mapBytes(r)
	}
	return n
}

func mapBytes(m map[string]string) int64 {
	if m == nil {
		return 0
	}
	n := int64(mapOverhead)
	for k, v := range m {
		n += kvOverhead + int64(len(k)) + int64(len(v))
	}
	return n
}
