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
	if err := m.sendEmail(ctx, sendTo, req); err != nil {
		return err
	}
	log.Info().Str("email", sendTo).Msg("Welcome email sent successfully")
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
	if err := m.sendEmail(ctx, sendTo, req); err != nil {
		return err
	}
	log.Info().Str("email", sendTo).Msg("Password reset email sent successfully")
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
	if err := m.sendEmail(ctx, sendTo, req); err != nil {
		return err
	}
	log.Info().Str("email", sendTo).Msg("Email verification OTP sent successfully")
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
	return m.sendEmail(ctx, sendTo, req)
}

// SendPendingDeleteEmail notifies the user their account is pending deletion.
func (m *Mailtrap) SendPendingDeleteEmail(ctx context.Context, sendTo string) error {
	c := content.PendingDeleteContent()
	req := SendEmailRequest{
		From:    From{Email: "moddns@demomailtrap.com", Name: "modDNS Team"},
		To:      []To{{Email: sendTo}},
		Subject: c.Subject,
		Text:    c.Plain,
		Html:    c.Html,
	}
	return m.sendEmail(ctx, sendTo, req)
}

// Verify checks if email provided is valid
func (m *Mailtrap) Verify(email string) error {
	initVerRes, err := m.verifier.Verify(email)
	if err != nil {
		fmt.Println("verify email address failed, error is: ", err)
		return err
	}
	if !initVerRes.Syntax.Valid {
		log.Debug().Str("email", email).Msg("email address syntax is invalid")
		if initVerRes.Suggestion != "" {
			log.Debug().Str("email", email).Str("suggested_domain", initVerRes.Suggestion).Msg("suggested domain")
		}
		return err
	}

	if initVerRes.Disposable {
		// TODO: decide whether disposable emails should be allowed
		log.Debug().Str("email", email).Msg(ErrEmailIsDisposable)
		return err
	}

	// TODO: decide wheter smtp verification is needed

	return nil
}

func (m *Mailtrap) sendEmail(ctx context.Context, email string, sendEmailReq SendEmailRequest) error {
	payload, err := json.Marshal(sendEmailReq)
	if err != nil {
		log.Err(err).Str("email", email).Msg("Failed to marshal email")
		return err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, m.sendEndpoint, bytes.NewReader(payload))
	if err != nil {
		log.Err(err).Str("email", email).Msg("Failed to create request")
		return err
	}
	req.Header.Add("Authorization", fmt.Sprintf("Bearer %s", m.authToken))
	req.Header.Add("Content-Type", "application/json")

	res, err := m.httpClient.Do(req) //nolint:gosec // G704 - URL is internally configured
	if err != nil {
		log.Err(err).Str("email", email).Msg("Failed to send http request")
		return err
	}
	defer res.Body.Close()

	responseBody, err := io.ReadAll(res.Body)
	if err != nil {
		log.Err(err).Str("email", email).Msg("Failed to read response body")
		return err
	}

	switch res.StatusCode {
	case http.StatusOK:
		break
	case http.StatusBadRequest:
		var errRes SendEmailErrors
		if err = json.Unmarshal(responseBody, &errRes); err != nil {
			log.Err(err).Str("email", email).Msg("Failed to unmarshal error response body")
			return err
		}
		log.Error().Err(err).Str("email", email).Strs("errors", errRes.Errors).Msg(ErrFailedToSendEmail)
		return errors.New(ErrFailedToSendEmail)
	default:
		err = errors.New(string(responseBody))
		log.Err(err).Str("email", email).Msg("Unknown send email error")
		return err
	}

	var response SendEmailResponse
	if err = json.Unmarshal(responseBody, &response); err != nil {
		log.Err(err).Str("email", email).Msg("Failed to unmarshal response body")
		return err
	}
	log.Debug().Str("response body", string(responseBody))
	return nil
}
