package dns

import (
	"encoding/json"
	"fmt"
	"net"
	"regexp"
	"strings"
	"time"

	"github.com/dnscheck/cache"
	"github.com/dnscheck/internal/maxmind"
	"github.com/miekg/dns"
	"github.com/rs/zerolog/log"
)

const (
	// SubdomainRegexPattern validates the dnscheck probe label: 12 alphanumeric
	// chars (nanoid). A "-suffix" is tolerated for clients still running the
	// previous frontend bundle, which appended the profile ID.
	SubdomainRegexPattern          = `^[a-zA-Z0-9]{12}(-[a-zA-Z0-9-]+)?$`
	ProfileIdAdditionalSectionCode = 0xfeed
	TTL                            = 300
)

var subdomainRegex = regexp.MustCompile(SubdomainRegexPattern)

type Handler struct {
	srv *DNSServer
}

func (h *Handler) ServeDNS(w dns.ResponseWriter, r *dns.Msg) {
	// Runs in a goroutine per packet, so an unrecovered panic would terminate the
	// process.
	defer func() {
		if rec := recover(); rec != nil {
			log.Error().Interface("panic", rec).
				Msg("Recovered from panic while serving DNS request")
		}
	}()

	// QDCOUNT (RFC 1035 §4.1.1) is a header field and is not validated against the
	// body, so a message can declare a question yet carry none. That unpacks
	// without error, leaving Question empty here.
	if len(r.Question) == 0 {
		log.Debug().Msg("Rejecting DNS request with no question section")
		m := new(dns.Msg)
		m.SetRcode(r, dns.RcodeFormatError)
		if err := w.WriteMsg(m); err != nil {
			log.Error().Err(err).Msg("Failed to write FORMERR response")
		}
		return
	}

	log.Debug().Str("protocol", w.RemoteAddr().Network()).Str("qtype", dns.Type(r.Question[0].Qtype).String()).Msg("Received DNS request")

	msg := dns.Msg{}
	msg.SetReply(r)
	switch r.Question[0].Qtype {
	case dns.TypeA:
		msg.Authoritative = true

		domain := strings.ToLower(msg.Question[0].Name)

		if strings.Contains(domain, h.srv.Config.Server.Domain) {
			subdomain := strings.Split(domain, ".")[0]

			if !subdomainRegex.MatchString(subdomain) {
				log.Warn().Msg("Unidentified subdomain")
				return
			}

			record := DNSLogRecord{}

			clientAddr, err := clientIP(w.RemoteAddr())
			if err != nil {
				log.Warn().Err(err).Msg("Cannot determine client IP address")
				return
			}

			// A failed lookup degrades to "no ASN information"; the IP-range check
			// below still decides the status and the answer is still written.
			lookupData, err := h.srv.GeoLookup.GetGeoLookup(clientAddr.String())
			if err != nil || lookupData == nil {
				log.Error().Err(err).Msg("GeoIP lookup failed, continuing without ASN")
				lookupData = &maxmind.GeoLookup{}
			}

			// decide whether IP address or ASN is from modDNS
			isOurIPRange := h.srv.Config.Server.ContainsIP(clientAddr)
			isOurASN := lookupData.ASN != 0 && lookupData.ASN == h.srv.Config.Server.ASN
			log.Trace().Bool("isOurIPRange", isOurIPRange).Bool("isOurASN", isOurASN).
				Msg("Checking if IP address or ASN is from our range")
			if isOurIPRange || isOurASN {
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
				log.Error().Err(err).Msg("Failed to save record")
			}
			log.Debug().Msg("Record saved")
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

// clientIP returns the transport-level source address of the query. It is read
// straight from the socket address and never resolved.
func clientIP(addr net.Addr) (net.IP, error) {
	switch a := addr.(type) {
	case *net.UDPAddr:
		return a.IP, nil
	case *net.TCPAddr:
		return a.IP, nil
	default:
		return nil, fmt.Errorf("unsupported remote address type %T", addr)
	}
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
