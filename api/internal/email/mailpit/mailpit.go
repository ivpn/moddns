package mailpit

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"

	"github.com/ivpn/dns/api/internal/email/content"
	"github.com/rs/zerolog/log"
)

const (
	WelcomeEmail  = "welcome_email"
	PasswordReset = "password_reset"
)

var (
	ErrFailedToSendEmail = "failed to send email"
)

// Mailpit is a struct that represents the Mailpit email service
type Mailpit struct {
	httpClient  *http.Client
	serverName  string
	apiEndpoint string
}

// NewMailpit creates a new Mailpit instance
func NewMailpit(serverName string) *Mailpit {
	mailpitSvc := "email"
	mailpitPort := "8025"
	apiEndpoint := fmt.Sprintf("http://%s:%s/api/v1/send", mailpitSvc, mailpitPort) // Default Mailpit API endpoint
	return &Mailpit{
		serverName:  serverName,
		httpClient:  &http.Client{},
		apiEndpoint: apiEndpoint,
	}
}

type Email struct {
	Email string `json:"Email"`
	Name  string `json:"Name"`
}

// mailpitSendRequest represents the payload for sending an email via Mailpit
type mailpitSendRequest struct {
	From    Email   `json:"From"`
	To      []Email `json:"To"`
	Subject string  `json:"Subject"`
	Text    string  `json:"Text"`
}

// SendWelcomeEmail sends a welcome email to the user using Mailpit
func (m *Mailpit) SendWelcomeEmail(ctx context.Context, sendTo, _ string) error {
	c := content.WelcomeContent(fmt.Sprintf("%s/home", m.serverName), fmt.Sprintf("%s/account-preferences", m.serverName))
	reqBody := mailpitSendRequest{
		From:    Email{Email: "info@moddns.net", Name: "modDNS"},
		To:      []Email{{Email: sendTo, Name: "User"}},
		Subject: c.Subject,
		Text:    c.Plain,
	}
	return m.sendEmail(ctx, sendTo, reqBody)
}

// SendPasswordResetEmail sends a password reset email to the user using Mailpit
func (m *Mailpit) SendPasswordResetEmail(ctx context.Context, sendTo, passwordResetToken string) error {
	c := content.PasswordResetContent(fmt.Sprintf("%s/reset-password/%s", m.serverName, passwordResetToken))
	reqBody := mailpitSendRequest{
		From:    Email{Email: "info@moddns.net", Name: "modDNS"},
		To:      []Email{{Email: sendTo, Name: "User"}},
		Subject: c.Subject,
		Text:    c.Plain,
	}
	return m.sendEmail(ctx, sendTo, reqBody)
}

// SendEmailVerificationOTP sends the OTP code to the user.
func (m *Mailpit) SendEmailVerificationOTP(ctx context.Context, sendTo, otp string) error {
	c := content.EmailVerificationOTPContent(otp)
	reqBody := mailpitSendRequest{
		From:    Email{Email: "info@moddns.net", Name: "modDNS"},
		To:      []Email{{Email: sendTo, Name: "User"}},
		Subject: c.Subject,
		Text:    c.Plain,
	}
	return m.sendEmail(ctx, sendTo, reqBody)
}

// SendSubscriptionExpiryEmail notifies the user their subscription has expired.
func (m *Mailpit) SendSubscriptionExpiryEmail(ctx context.Context, sendTo string) error {
	c := content.SubscriptionExpiryContent()
	reqBody := mailpitSendRequest{
		From:    Email{Email: "info@moddns.net", Name: "modDNS"},
		To:      []Email{{Email: sendTo, Name: "User"}},
		Subject: c.Subject,
		Text:    c.Plain,
	}
	return m.sendEmail(ctx, sendTo, reqBody)
}

// sendEmail sends an email using the Mailpit API
func (m *Mailpit) sendEmail(ctx context.Context, email string, reqBody mailpitSendRequest) error {
	payload, err := json.Marshal(reqBody)
	if err != nil {
		log.Err(err).Str("email", email).Msg("Mailpit: Failed to marshal email payload")
		return err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, m.apiEndpoint, bytes.NewReader(payload))
	if err != nil {
		log.Err(err).Str("email", email).Msg("Mailpit: Failed to create request")
		return err
	}
	req.Header.Add("Content-Type", "application/json")
	// Mailpit does not require authentication by default, but if needed:
	// req.Header.Add("Authorization", fmt.Sprintf("Bearer %s", m.authToken))

	res, err := m.httpClient.Do(req) //nolint:gosec // G704 - URL is internally configured
	if err != nil {
		log.Err(err).Str("email", email).Msg("Mailpit: Failed to send HTTP request")
		return err
	}
	defer res.Body.Close()

	responseBody, err := io.ReadAll(res.Body)
	if err != nil {
		log.Err(err).Str("email", email).Msg("Mailpit: Failed to read response body")
		return err
	}

	if res.StatusCode != http.StatusOK {
		log.Error().Str("email", email).Int("status", res.StatusCode).Str("body", string(responseBody)).Msg(ErrFailedToSendEmail)
		return errors.New(ErrFailedToSendEmail)
	}

	log.Info().Str("email", email).Msg("Mailpit: Email sent successfully")
	return nil
}

// Verify checks if the provided email is valid (basic syntax check)
func (m *Mailpit) Verify(email string) error {
	// For demonstration, only check for empty string. Extend as needed.
	if email == "" {
		return errors.New("email is empty")
	}
	// Optionally, add regex or use a library for more robust validation.
	return nil
}
