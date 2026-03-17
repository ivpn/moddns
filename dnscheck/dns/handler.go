package dns

import (
	"encoding/json"
	"errors"
	"net"
	"regexp"
	"strings"
	"time"

	"github.com/dnscheck/cache"
	"github.com/miekg/dns"
	"github.com/rs/zerolog/log"
)

const (
	// SubdomainRegexPattern validates the expected dnscheck subdomain format:
	// 12 alphanumeric chars (nanoid), a dash, then the profile ID.
	SubdomainRegexPattern          = `^[a-zA-Z0-9]{12}-[a-zA-Z0-9-]+$`
	ProfileIdAdditionalSectionCode = 0xfeed
	TTL                            = 300
)

type Handler struct {
	srv *DNSServer
}

func (h *Handler) ServeDNS(w dns.ResponseWriter, r *dns.Msg) {
	log.Debug().Str("protocol", w.RemoteAddr().Network()).Str("qtype", dns.Type(r.Question[0].Qtype).String()).Msgf("Received DNS request: %s", r.Question[0].Name)

	msg := dns.Msg{}
	msg.SetReply(r)
	switch r.Question[0].Qtype {
	case dns.TypeA:
		msg.Authoritative = true

		domain := strings.ToLower(msg.Question[0].Name)

		if strings.Contains(domain, h.srv.Config.Server.Domain) {
			subdomain := strings.Split(domain, ".")[0]

			// Regex to identify the subdomain with the first part being exactly 12 characters
			matched, err := regexp.MatchString(SubdomainRegexPattern, subdomain)
			if err != nil {
				log.Error().Err(err).Msg("Failed to compile regex")
				return
			}

			if !matched {
				log.Warn().Str("subdomain", subdomain).Msg("Unidentified subdomain")
				return
			}

			record := DNSLogRecord{}

			udp := strings.HasPrefix(w.RemoteAddr().Network(), "udp")
			var extractionMode string
			if udp {
				extractionMode = "udp"
			} else {
				extractionMode = "tcp"
			}
			IPAddress, _, err := h.extractIPAddressAndHostname(w, extractionMode)
			if err != nil {
				log.Warn().Err(err).Msgf("Error resolving address %s %s, defaulting to hostname None", w.RemoteAddr().Network(), w.RemoteAddr().String())
				return
			}

			lookupData, err := h.srv.GeoLookup.GetGeoLookup(IPAddress)
			if err != nil {
				log.Error().Err(err).Msgf("Error getting GeoLookup for %s", IPAddress)
			}

			record.IPAddress = IPAddress
			record.ASN = lookupData.ASN
			record.ASNOrganization = lookupData.ASNOrganization

			// decide whether IP address or ASN is from modDNS
			log.Trace().Bool("isOurIPRange", strings.HasPrefix(IPAddress, h.srv.Config.Server.IPRange)).
				Bool("isOurASN", lookupData.ASN == h.srv.Config.Server.ASN).
				Msg("Checking if IP address or ASN is from our range")
			if strings.HasPrefix(IPAddress, h.srv.Config.Server.IPRange) || lookupData.ASN == h.srv.Config.Server.ASN {
				profileId := h.extractConfiguredProfileId(r)
				record.Status = StatusConfigured
				record.ProfileId = profileId
			} else {
				record.Status = StatusUnconfigured
			}

			recordBytes, err := json.Marshal(record)
			if err != nil {
				log.Error().Err(err).Msg("Failed to marshal record")
			}
			cacheKey := cache.HMACKey(h.srv.Config.Cache.HMACKey, subdomain)
			if err = h.srv.Cache.SaveQueryData(cacheKey, recordBytes); err != nil {
				log.Error().Err(err).Str("ID", subdomain).Msg("Failed to save record")
			}
			log.Debug().Str("ID", subdomain).Msg("Record saved")
		}

		msg.Answer = append(msg.Answer, &dns.A{
			Hdr: dns.RR_Header{Name: msg.Question[0].Name, Rrtype: dns.TypeA, Class: dns.ClassINET, Ttl: TTL},
			A:   net.ParseIP(h.srv.Config.Server.IPAddress),
		})
		msg.Ns = append(msg.Ns, &dns.NS{
			Hdr: dns.RR_Header{Name: h.srv.Config.Server.Domain + ".", Rrtype: dns.TypeNS, Class: dns.ClassINET, Ttl: TTL},
			Ns:  "ns1." + h.srv.Config.Server.Domain + ".",
		})
		msg.Ns = append(msg.Ns, &dns.NS{
			Hdr: dns.RR_Header{Name: h.srv.Config.Server.Domain + ".", Rrtype: dns.TypeNS, Class: dns.ClassINET, Ttl: TTL},
			Ns:  "ns2." + h.srv.Config.Server.Domain + ".",
		})
		msg.Extra = append(msg.Extra, &dns.A{
			Hdr: dns.RR_Header{Name: "ns1." + h.srv.Config.Server.Domain + ".", Rrtype: dns.TypeA, Class: dns.ClassINET, Ttl: TTL},
			A:   net.ParseIP(h.srv.Config.Server.IPAddress),
		})
		msg.Extra = append(msg.Extra, &dns.A{
			Hdr: dns.RR_Header{Name: "ns2." + h.srv.Config.Server.Domain + ".", Rrtype: dns.TypeA, Class: dns.ClassINET, Ttl: TTL},
			A:   net.ParseIP(h.srv.Config.Server.IPAddress),
		})
	case dns.TypeNS:
		msg.Authoritative = true
		msg.Ns = h.createSOA()
		msg.Answer = append(msg.Answer, &dns.NS{
			Hdr: dns.RR_Header{Name: h.srv.Config.Server.Domain + ".", Rrtype: dns.TypeNS, Class: dns.ClassINET, Ttl: TTL},
			Ns:  "ns1." + h.srv.Config.Server.Domain + ".",
		})
		msg.Answer = append(msg.Answer, &dns.NS{
			Hdr: dns.RR_Header{Name: h.srv.Config.Server.Domain + ".", Rrtype: dns.TypeNS, Class: dns.ClassINET, Ttl: TTL},
			Ns:  "ns2." + h.srv.Config.Server.Domain + ".",
		})
		msg.Extra = append(msg.Extra, &dns.A{
			Hdr: dns.RR_Header{Name: "ns1." + h.srv.Config.Server.Domain + ".", Rrtype: dns.TypeA, Class: dns.ClassINET, Ttl: TTL},
			A:   net.ParseIP(h.srv.Config.Server.IPAddress),
		})
		msg.Extra = append(msg.Extra, &dns.A{
			Hdr: dns.RR_Header{Name: "ns2." + h.srv.Config.Server.Domain + ".", Rrtype: dns.TypeA, Class: dns.ClassINET, Ttl: TTL},
			A:   net.ParseIP(h.srv.Config.Server.IPAddress),
		})
	case dns.TypeSOA:
		msg.Authoritative = true
		msg.Answer = h.createSOA()
		msg.Ns = append(msg.Ns, &dns.NS{
			Hdr: dns.RR_Header{Name: h.srv.Config.Server.Domain + ".", Rrtype: dns.TypeNS, Class: dns.ClassINET, Ttl: TTL},
			Ns:  "ns1." + h.srv.Config.Server.Domain + ".",
		})
		msg.Ns = append(msg.Ns, &dns.NS{
			Hdr: dns.RR_Header{Name: h.srv.Config.Server.Domain + ".", Rrtype: dns.TypeNS, Class: dns.ClassINET, Ttl: TTL},
			Ns:  "ns2." + h.srv.Config.Server.Domain + ".",
		})
		msg.Extra = append(msg.Extra, &dns.A{
			Hdr: dns.RR_Header{Name: "ns1." + h.srv.Config.Server.Domain + ".", Rrtype: dns.TypeA, Class: dns.ClassINET, Ttl: TTL},
			A:   net.ParseIP(h.srv.Config.Server.IPAddress),
		})
		msg.Extra = append(msg.Extra, &dns.A{
			Hdr: dns.RR_Header{Name: "ns2." + h.srv.Config.Server.Domain + ".", Rrtype: dns.TypeA, Class: dns.ClassINET, Ttl: TTL},
			A:   net.ParseIP(h.srv.Config.Server.IPAddress),
		})
	default:
		msg.Ns = h.createSOA()
	}
	w.WriteMsg(&msg)
}

func (h *Handler) extractConfiguredProfileId(r *dns.Msg) (profileId string) {
	// Extract custom data from the additional section
	for _, extra := range r.Extra {
		if opt, ok := extra.(*dns.OPT); ok {
			for _, option := range opt.Option {
				if edns0Local, ok := option.(*dns.EDNS0_LOCAL); ok {
					if edns0Local.Code == ProfileIdAdditionalSectionCode {
						profileId = string(edns0Local.Data)
						return
					}
				}
			}
		}
	}
	return ""
}

func (h *Handler) createSOA() []dns.RR {
	dom := dns.Fqdn(h.srv.Config.Server.Domain + ".")

	return []dns.RR{
		&dns.SOA{
			Hdr: dns.RR_Header{
				Name:   dom,
				Rrtype: dns.TypeSOA,
				Class:  dns.ClassINET,
				Ttl:    TTL},
			Ns:      "ns1." + dom,
			Mbox:    "hostmaster." + dom,
			Serial:  uint32(time.Now().Truncate(time.Hour).Unix()),
			Refresh: 28800,
			Retry:   7200,
			Expire:  604800,
			Minttl:  TTL,
		},
	}
}

func (h *Handler) extractIPAddressAndHostname(w dns.ResponseWriter, extractionMode string) (IPAddress string, hostname string, err error) {
	switch extractionMode {
	case "udp":
		addr, err := net.ResolveUDPAddr(w.RemoteAddr().Network(), w.RemoteAddr().String())
		if err != nil {
			return "", "", err
		}
		IPAddress = addr.IP.String()
		hostnames, err := net.LookupAddr(IPAddress)
		if err == nil && len(hostnames) > 0 {
			hostname = hostnames[0]
		}
	case "tcp":
		addr, err := net.ResolveTCPAddr(w.RemoteAddr().Network(), w.RemoteAddr().String())
		if err != nil {
			return "", "", err
		}
		IPAddress = addr.IP.String()
		hostnames, err := net.LookupAddr(IPAddress)
		if err == nil && len(hostnames) > 0 {
			hostname = hostnames[0]
		}
	default:
		return "", "", errors.New("invalid extraction mode")
	}

	return IPAddress, hostname, nil
}

func FindStringSubmatchMap(rs string, s string) map[string]string {
	r := regexp.MustCompile(rs)

	captures := make(map[string]string)

	match := r.FindStringSubmatch(s)
	if match == nil {
		return captures
	}

	for i, name := range r.SubexpNames() {
		// Ignore the whole regexp match and unnamed groups
		if i == 0 || name == "" {
			continue
		}

		captures[name] = match[i]

	}

	return captures
}
