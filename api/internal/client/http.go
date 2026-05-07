package client

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/gofiber/fiber/v2"
	"github.com/ivpn/dns/api/config"
	"github.com/ivpn/dns/api/model"
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
		body, _ := json.Marshal(map[string]string{"uuid": subID, "service": "dns"})
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
	} else {
		log.Debug().Msg("No signup webhook configured, skipping")
	}
	return nil
}

// GetPreauth fetches a preauth entry from the external preauth service.
func (h Http) GetPreauth(id string) (model.Preauth, error) {
	req := fiber.Get(h.Cfg.PreauthURL + "/" + id)
	req.Set("Content-Type", "application/json")
	req.Set("Accept", "application/json")
	req.Set("Authorization", "Bearer "+h.Cfg.PreauthPSK)

	status, body, errs := req.Bytes()
	if len(errs) > 0 {
		log.Error().Interface("errors", errs).Msg("Error calling preauth service")
		return model.Preauth{}, errors.New("error calling preauth service")
	}

	if status != http.StatusOK {
		log.Error().Int("status", status).Msg("Error calling preauth service")
		return model.Preauth{}, errors.New("error response from preauth service")
	}

	var preauth model.Preauth
	if err := json.Unmarshal(body, &preauth); err != nil {
		log.Error().Err(err).Msg("Error parsing preauth response")
		return model.Preauth{}, errors.New("error parsing preauth response")
	}

	return preauth, nil
}
