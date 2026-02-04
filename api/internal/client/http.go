package client

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/gofiber/fiber/v2"
	"github.com/ivpn/dns/api/config"
	"github.com/rs/zerolog/log"
)

type Http struct {
	Cfg config.APIConfig
}

func New(cfg config.APIConfig) *Http {
	return &Http{
		Cfg: cfg,
	}
}

func (h Http) SignupWebhook(subID string) error {
	log.Debug().Msg("Calling signup webhook")
	if h.Cfg.SignupWebhookURL != "" {
		req := fiber.Post(h.Cfg.SignupWebhookURL)
		req.Set("Content-Type", "application/json")
		req.Set("Accept", "application/json")
		req.Set("Authorization", "Bearer "+h.Cfg.SignupWebhookPSK)
		body, _ := json.Marshal(map[string]string{"uuid": subID})
		req.Body(body)

		status, _, err := req.Bytes()
		log.Info().Int("status", status).Msgf("Called signup webhook")
		if err != nil {
			log.Error().Interface("error", err).Msg("Error calling signup webhook")
			return errors.New("error calling signup webhook")
		}

		if status != http.StatusOK {
			log.Error().Int("status", status).Msgf("Error calling signup webhook")
			return errors.New("error response from signup webhook")
		}
	}
	log.Debug().Msg("No signup webhook configured, skipping")
	return nil
}
