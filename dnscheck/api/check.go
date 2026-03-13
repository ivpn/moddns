package api

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"

	"github.com/dnscheck/cache"
	"github.com/dnscheck/dns"
	"github.com/gofiber/fiber/v2"
	"github.com/rs/zerolog/log"
)

var subdomainRegex = regexp.MustCompile(dns.SubdomainRegexPattern)

func (s *APIServer) DnsCheck() fiber.Handler {
	handler := func(c *fiber.Ctx) error {
		host := c.Hostname()
		log.Debug().Str("host", host).Msg("Host")
		hostParts := strings.Split(host, ".")
		if len(hostParts) < 2 {
			log.Error().Msg(ErrInvalidHostHeader)
			err := fmt.Errorf("invalid header: %s", host)
			return HandleError(c, err, ErrInvalidHostHeader)
		}

		subdomain := strings.ToLower(hostParts[0])
		if !subdomainRegex.MatchString(subdomain) {
			log.Warn().Str("subdomain", subdomain).Msg("Invalid subdomain format")
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid request"})
		}

		// get data from cache
		cacheKey := cache.HMACKey(s.Config.Cache.HMACKey, subdomain)
		log.Debug().Str("ID", subdomain).Msg("Getting query data")
		data, err := s.Cache.GetQueryData(cacheKey)
		if err != nil {
			return HandleError(c, err, ErrFailedToGetQueryData)
		}

		// Delete-on-read: each subdomain is single-use (frontend generates a fresh
		// nanoid per poll), so delete immediately to minimize the replay window.
		if delErr := s.Cache.DeleteQueryData(cacheKey); delErr != nil {
			log.Warn().Err(delErr).Str("ID", subdomain).Msg("Failed to delete cache entry after read")
		}

		var dnsRecord dns.DNSLogRecord
		if err = json.Unmarshal(data, &dnsRecord); err != nil {
			log.Error().Err(err).Msg(ErrFailedToUnmarshalRecord)
			return HandleError(c, err, ErrFailedToUnmarshalRecord)
		}

		return c.Status(200).JSON(dns.DNSCheckResponse{
			Status:    dnsRecord.Status,
			ProfileId: dnsRecord.ProfileId,
		})
	}
	return handler
}
