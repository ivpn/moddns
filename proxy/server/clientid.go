package server

import (
	"crypto/tls"
	"fmt"
	"net/url"
	"path"
	"strings"

	"github.com/AdguardTeam/dnsproxy/proxy"
	"github.com/AdguardTeam/golibs/errors"
	"github.com/AdguardTeam/golibs/log"
	"github.com/AdguardTeam/golibs/netutil"
	zerolog "github.com/rs/zerolog/log"

	"github.com/ivpn/dns/libs/deviceid"
)

// profileIDMinLength holds the minimum length considered valid for profile IDs.
// Configurable via PROFILE_ID_MIN_LENGTH env in both API and proxy; default 10.
var profileIDMinLength = 10

// ValidateClientID returns an error if id is not a valid ClientID.
//
// Keep in sync with [client.ValidateClientID].
func ValidateClientID(id string) (err error) {
	err = netutil.ValidateHostnameLabel(id)
	if err != nil {
		// Replace the domain name label wrapper with our own.
		return fmt.Errorf("invalid clientid %q: %w", id, errors.Unwrap(err))
	}

	return nil
}

// SanitizeDeviceIdForDNS kept for backward compatibility in DoT/DoQ path using shared deviceid lib.
func SanitizeDeviceIdForDNS(deviceId string) string { return deviceid.SanitizeForDNS(deviceId) }

// isValidProfileID checks if a string could be a valid profile ID
func isValidProfileID(s string) bool {
	// Profile IDs are typically UUIDs or alphanumeric strings
	if len(s) < profileIDMinLength {
		return false
	}
	for _, r := range s {
		if !((r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9')) {
			return false
		}
	}
	return true
}

// clientIDFromClientServerName extracts and validates a ClientID and device ID.
// hostSrvName is the server name of the host.  cliSrvName is the server name as sent by the
// client.  When strict is true, and client and host server name don't match,
// clientIDFromClientServerName will return an error.
// Format: {device_id}-{profile_id}.domain or just {profile_id}.domain
func clientIDFromClientServerName(
	hostSrvName string,
	cliSrvName string,
	strict bool,
	proto proxy.Proto,
) (clientID, deviceId string, err error) {
	if hostSrvName == cliSrvName {
		return "", "", nil
	}

	if !netutil.IsImmediateSubdomain(cliSrvName, hostSrvName) {
		if !strict {
			return "", "", nil
		}

		return "", "", fmt.Errorf(
			"client server name %q doesn't match host server name %q",
			cliSrvName,
			hostSrvName,
		)
	}

	subdomain := cliSrvName[:len(cliSrvName)-len(hostSrvName)-1]

	// Parse device name and profile ID from subdomain
	// Format: {device_name}-{profile_id} or just {profile_id}
	parts := strings.Split(subdomain, "-")
	if len(parts) < 1 {
		return "", "", fmt.Errorf("invalid subdomain format: %s", subdomain)
	}

	// Find profile ID (should be the last part that's alphanumeric)
	var profileIDIndex int = -1
	for i := len(parts) - 1; i >= 0; i-- {
		if isValidProfileID(parts[i]) {
			profileIDIndex = i
			break
		}
	}

	if profileIDIndex == -1 {
		return "", "", fmt.Errorf("no valid profile ID found in subdomain: %s", subdomain)
	}

	clientID = parts[profileIDIndex]

	// Device ID is everything before the profile ID
	if profileIDIndex > 0 {
		deviceIdParts := parts[:profileIDIndex]
		// Domain representation (no spaces; original spaces encoded as -- already)
		joined := strings.Join(deviceIdParts, "-")

		// Sanitize domain representation -> label form (legacy) then convert back to logical.
		joined = SanitizeDeviceIdForDNS(joined)
		deviceId = deviceid.DecodeLabel(joined)
		deviceId = deviceid.Normalize(deviceId)
	}

	err = ValidateClientID(clientID)
	if err != nil {
		// Don't wrap the error, because it's informative enough as is.
		return "", "", err
	}

	return strings.ToLower(clientID), deviceId, nil
}

// clientIDFromDNSContextHTTPS extracts the client's ID and device ID from the path of the
// client's DNS-over-HTTPS request.
// To test: https://ivpndns.com:443/dns-query/123/device-id
func clientIDFromDNSContextHTTPS(pctx *proxy.DNSContext) (clientID, deviceId string, err error) {
	r := pctx.HTTPRequest
	if r == nil {
		return "", "", fmt.Errorf(
			"proxy ctx http request of proto %s is nil",
			pctx.Proto,
		)
	}

	origPath := r.URL.Path
	parts := strings.Split(path.Clean(origPath), "/")
	if parts[0] == "" {
		parts = parts[1:]
	}

	if len(parts) == 0 || parts[0] != "dns-query" {
		return "", "", fmt.Errorf("clientid check: invalid path %q", origPath)
	}

	switch len(parts) {
	case 1:
		// Just /dns-query, no ClientID.
		return "", "", nil
	case 2:
		// /dns-query/{profile_id}
		clientID = parts[1]
		deviceId = ""
	case 3:
		// /dns-query/{profile_id}/{device_id}
		clientID = parts[1]
		deviceId, err = url.QueryUnescape(parts[2])
		if err != nil {
			return "", "", fmt.Errorf("failed to decode device ID: %w", err)
		}
		// Normalize + truncate for DoH path.
		deviceId = deviceid.Normalize(deviceId)
	default:
		return "", "", fmt.Errorf("clientid check: invalid path %q: too many parts", origPath)
	}

	err = ValidateClientID(clientID)
	if err != nil {
		return "", "", fmt.Errorf("clientid check: %w", err)
	}

	return strings.ToLower(clientID), deviceId, nil
}

// tlsConn is a narrow interface for *tls.Conn to simplify testing.
type tlsConn interface {
	ConnectionState() (cs tls.ConnectionState)
}

// clientIDFromDNSContext extracts the client's ID and device ID from the server name of the
// client's DoT or DoQ request or the path of the client's DoH.  If the protocol
// is not one of these, clientID is an empty string and err is nil.
func (s *Server) clientIDFromDNSContext(pctx *proxy.DNSContext) (clientID, deviceId string, err error) {
	proto := pctx.Proto
	if proto == proxy.ProtoHTTPS {
		clientID, deviceId, err = clientIDFromDNSContextHTTPS(pctx)
		if err != nil {
			return "", "", fmt.Errorf("checking url: %w", err)
		} else if clientID != "" {
			return clientID, deviceId, nil
		}

		// Go on and check the domain name as well.
	} else if proto != proxy.ProtoTLS && proto != proxy.ProtoQUIC {
		return "", "", nil
	}

	hostSrvName := s.Config.Server.Name
	if hostSrvName == "" {
		return "", "", nil
	}

	cliSrvName, err := clientServerName(pctx, proto)
	if err != nil {
		return "", "", err
	}

	clientID, deviceId, err = clientIDFromClientServerName(
		hostSrvName,
		cliSrvName,
		false, // TODO: check
		proto,
	)
	zerolog.Info().Str("cliSrvName", cliSrvName).Str("hostSrvName", hostSrvName).Str("clientID", clientID).Str("deviceId", deviceId).Msg("client and server names ")
	if err != nil {
		return "", "", fmt.Errorf("clientid check: %w", err)
	}

	return clientID, deviceId, nil
}

// clientServerName returns the TLS server name based on the protocol.  For
// DNS-over-HTTPS requests, it will return the hostname part of the Host header
// if there is one.
func clientServerName(pctx *proxy.DNSContext, proto proxy.Proto) (srvName string, err error) {
	from := "tls conn"

	switch proto {
	case proxy.ProtoHTTPS:
		r := pctx.HTTPRequest
		if connState := r.TLS; connState != nil {
			srvName = connState.ServerName
		} else if r.Host != "" {
			var host string
			host, err = netutil.SplitHost(r.Host)
			if err != nil {
				return "", fmt.Errorf("parsing host: %w", err)
			}

			srvName = host
			from = "host header"
		}
	case proxy.ProtoQUIC:
		qConn := pctx.QUICConnection
		if qConn == nil {
			return "", fmt.Errorf("pctx conn of proto %s is nil", proto)
		}

		srvName = qConn.ConnectionState().TLS.ServerName
	case proxy.ProtoTLS:
		conn := pctx.Conn
		tc, ok := conn.(tlsConn)
		if !ok {
			return "", fmt.Errorf("pctx conn of proto %s is %T, want *tls.Conn", proto, conn)
		}

		srvName = tc.ConnectionState().ServerName
	}

	log.Debug("dnsforward: got client server name %q from %s", srvName, from)

	return srvName, nil
}
