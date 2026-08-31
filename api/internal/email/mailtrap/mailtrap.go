package mailtrap

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"

	emailverifier "github.com/AfterShip/email-verifier"
	"github.com/ivpn/dns/api/internal/email/content"
	"github.com/ivpn/dns/libs/logging"
	"github.com/rs/zerolog/log"
)

const (
	WelcomeEmail  = "welcome_email"
	PasswordReset = "password_reset"
)

var (
	ErrEmailIsDisposable = "email address is disposable"
	ErrFailedToSendEmail = "failed to send email"
)

// Mailtrap is a struct that represents Mailtrap email service
type Mailtrap struct {
	httpClient   *http.Client
	serverName   string
	inboxId      string
	authToken    string
	verifier     *emailverifier.Verifier
	sendEndpoint string
}

// NewMailtrap creates a new Mailtrap instance
func NewMailtrap(serverName, inboxId, authToken string) *Mailtrap {
	verifier := emailverifier.NewVerifier().EnableDomainSuggest()
	sendEndpoint := fmt.Sprintf("https://sandbox.api.mailtrap.io/api/send/%s", inboxId)
	return &Mailtrap{
		serverName:   serverName,
		inboxId:      inboxId,
		authToken:    authToken,
		httpClient:   &http.Client{},
		verifier:     verifier,
		sendEndpoint: sendEndpoint,
	}
}

// SendWelcomeEmail sends a welcome email to the user
func (m *Mailtrap) SendWelcomeEmail(ctx context.Context, sendTo, _ string) error {
	c := content.WelcomeContent(fmt.Sprintf("%s/home", m.serverName), fmt.Sprintf("%s/account-preferences", m.serverName))
	req := SendEmailRequest{
		From:     From{Email: "moddns@demomailtrap.com", Name: "modDNS"},
		To:       []To{{Email: sendTo}},
		Subject:  c.Subject,
		Text:     c.Plain,
		Html:     c.Html,
		Category: WelcomeEmail,
	}
	if err := m.sendEmail(ctx, req); err != nil {
		return err
	}
	log.Info().Msg("Welcome email sent successfully")
	return nil
}

// SendPasswordResetEmail sends a password reset email to the user
func (m *Mailtrap) SendPasswordResetEmail(ctx context.Context, sendTo, passwordResetToken string) error {
	c := content.PasswordResetContent(fmt.Sprintf("%s/reset-password/%s", m.serverName, passwordResetToken))
	req := SendEmailRequest{
		From:     From{Email: "moddns@demomailtrap.com", Name: "modDNS Team"},
		To:       []To{{Email: sendTo}},
		Subject:  c.Subject,
		Text:     c.Plain,
		Html:     c.Html,
		Category: PasswordReset,
	}
	if err := m.sendEmail(ctx, req); err != nil {
		return err
	}
	log.Info().Msg("Password reset email sent successfully")
	return nil
}

// SendEmailVerificationOTP sends a 6-digit OTP code for email verification.
func (m *Mailtrap) SendEmailVerificationOTP(ctx context.Context, sendTo, otp string) error {
	c := content.EmailVerificationOTPContent(otp)
	req := SendEmailRequest{
		From:    From{Email: "moddns@demomailtrap.com", Name: "modDNS Team"},
		To:      []To{{Email: sendTo}},
		Subject: c.Subject,
		Text:    c.Plain,
		Html:    c.Html,
	}
	if err := m.sendEmail(ctx, req); err != nil {
		return err
	}
	log.Info().Msg("Email verification OTP sent successfully")
	return nil
}

// SendSubscriptionExpiryEmail notifies the user their subscription has expired.
func (m *Mailtrap) SendSubscriptionExpiryEmail(ctx context.Context, sendTo string) error {
	c := content.SubscriptionExpiryContent()
	req := SendEmailRequest{
		From:    From{Email: "moddns@demomailtrap.com", Name: "modDNS Team"},
		To:      []To{{Email: sendTo}},
		Subject: c.Subject,
		Text:    c.Plain,
		Html:    c.Html,
	}
	return m.sendEmail(ctx, req)
}

// SendInactiveEmail notifies the user their account has become inactive.
func (m *Mailtrap) SendInactiveEmail(ctx context.Context, sendTo string) error {
	c := content.InactiveContent()
	req := SendEmailRequest{
		From:    From{Email: "moddns@demomailtrap.com", Name: "modDNS Team"},
		To:      []To{{Email: sendTo}},
		Subject: c.Subject,
		Text:    c.Plain,
		Html:    c.Html,
	}
	return m.sendEmail(ctx, req)
}

// Verify checks if email provided is valid
func (m *Mailtrap) Verify(email string) error {
	initVerRes, err := m.verifier.Verify(email)
	if err != nil {
		log.Err(err).Msg("verify email address failed")
		return err
	}
	if !initVerRes.Syntax.Valid {
		log.Debug().Msg("email address syntax is invalid")
		if initVerRes.Suggestion != "" {
			log.Debug().Str("suggested_domain", initVerRes.Suggestion).Msg("suggested domain")
		}
		return err
	}

	if initVerRes.Disposable {
		// TODO: decide whether disposable emails should be allowed
		log.Debug().Msg(ErrEmailIsDisposable)
		return err
	}

	// TODO: decide wheter smtp verification is needed

	return nil
}

func (m *Mailtrap) sendEmail(ctx context.Context, sendEmailReq SendEmailRequest) error {
	payload, err := json.Marshal(sendEmailReq)
	if err != nil {
		log.Err(err).Msg("Failed to marshal email")
		return err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, m.sendEndpoint, bytes.NewReader(payload))
	if err != nil {
		log.Err(err).Msg("Failed to create request")
		return err
	}
	req.Header.Add("Authorization", fmt.Sprintf("Bearer %s", m.authToken))
	req.Header.Add("Content-Type", "application/json")

	res, err := m.httpClient.Do(req) //nolint:gosec // G704 - URL is internally configured
	if err != nil {
		log.Err(err).Msg("Failed to send http request")
		return err
	}
	defer res.Body.Close()

	responseBody, err := io.ReadAll(res.Body)
	if err != nil {
		log.Err(err).Msg("Failed to read response body")
		return err
	}

	switch res.StatusCode {
	case http.StatusOK:
		break
	case http.StatusBadRequest:
		var errRes SendEmailErrors
		if err = json.Unmarshal(responseBody, &errRes); err != nil {
			log.Err(err).Msg("Failed to unmarshal error response body")
			return err
		}
		// Provider validation errors can embed the recipient address.
		redacted := make([]string, len(errRes.Errors))
		for i, e := range errRes.Errors {
			redacted[i] = logging.RedactEmails(e)
		}
		log.Error().Strs("errors", redacted).Msg(ErrFailedToSendEmail)
		return errors.New(ErrFailedToSendEmail)
	default:
		err = errors.New(logging.RedactEmails(string(responseBody)))
		log.Err(err).Msg("Unknown send email error")
		return err
	}

	var response SendEmailResponse
	if err = json.Unmarshal(responseBody, &response); err != nil {
		log.Err(err).Msg("Failed to unmarshal response body")
		return err
	}
	return nil
}
