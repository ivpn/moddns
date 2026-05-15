// Package dohpath defines the DoH URL path layout shared between the proxy
// (which routes incoming DoH requests) and the api (which generates DNS Stamps
// pointing at those paths). A single source of truth prevents the two services
// from silently drifting if the path scheme ever changes.
package dohpath

import (
	"github.com/ivpn/dns/libs/deviceid"
)

const (
	// Segment is the first URL path segment served by the proxy DoH listener.
	// The proxy router matches against this constant (see proxy/server/clientid.go).
	Segment = "dns-query"

	// Prefix is the leading URL path before the profile id. Always begins
	// and ends with a slash so `Prefix + profileId` yields a valid path.
	Prefix = "/" + Segment + "/"
)

// For returns the DoH URL path for the given profile and optional device.
// The device id is URL-encoded per deviceid.EncodeURL (spaces become %20).
//
// Examples:
//
//	For("abc123def4", "")            → "/dns-query/abc123def4"
//	For("abc123def4", "Living Room") → "/dns-query/abc123def4/Living%20Room"
func For(profileId, deviceId string) string {
	p := Prefix + profileId
	if deviceId == "" {
		return p
	}
	return p + "/" + deviceid.EncodeURL(deviceId)
}
