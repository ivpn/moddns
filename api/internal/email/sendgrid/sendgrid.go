package sendgrid

import (
	"context"
	"errors"
	"fmt"
	"regexp"

	"github.com/ivpn/dns/api/internal/email/content"
	"github.com/rs/zerolog/log"
	"github.com/sendgrid/rest"
	sg "github.com/sendgrid/sendgrid-go"
	"github.com/sendgrid/sendgrid-go/helpers/mail"
)

// Template IDs removed – implementation now always sends plain/HTML emails without dynamic templates.

var (
	ErrFailedToSendEmail = errors.New("failed to send email")
)

// Option pattern for easier testing/injection
type Option func(*Mailer)

// WithClient allows providing custom sendgrid client (e.g., mock)
func WithClient(client interface {
	Send(*mail.SGMailV3) (*rest.Response, error)
}) Option {
	return func(m *Mailer) { m.client = client }
}

// Mailer implements email.Mailer for SendGrid
type Mailer struct {
	serverName string
	apiKey     string
	client     interface {
		Send(*mail.SGMailV3) (*rest.Response, error)
	}
	// simple email regex for Verify fallback (not production-grade)
	emailRegex *regexp.Regexp
}

// New creates new SendGrid mailer
// New creates a new SendGrid Mailer without template usage.
// Backwards compatibility: signature changed (removed template IDs); callers should pass only serverName, apiKey, fromEmail, fromName.
func New(serverName, apiKey string, opts ...Option) *Mailer {
	m := &Mailer{
		serverName: serverName,
		apiKey:     apiKey,
		client:     sg.NewSendClient(apiKey),
		emailRegex: regexp.MustCompile(`^[^@\s]+@[^@\s]+\.[^@\s]+$`),
	}
	for _, o := range opts {
		o(m)
	}
	return m
}

// SendWelcomeEmail sends welcome email with template dynamic data
func (m *Mailer) SendWelcomeEmail(ctx context.Context, sendTo, _ string) error {
	c := content.WelcomeContent(fmt.Sprintf("%s/home", m.serverName), fmt.Sprintf("%s/account-preferences", m.serverName))
	return m.sendBasic(ctx, sendTo, c.Subject, c.Plain, c.Html)
}

// SendEmailVerificationOTP sends a 6-digit OTP code for email verification.
func (m *Mailer) SendEmailVerificationOTP(ctx context.Context, sendTo, otp string) error {
	c := content.EmailVerificationOTPContent(otp)
	return m.sendBasic(ctx, sendTo, c.Subject, c.Plain, c.Html)
}

// SendPasswordResetEmail sends password reset email with template dynamic data
func (m *Mailer) SendPasswordResetEmail(ctx context.Context, sendTo, passwordResetToken string) error {
	c := content.PasswordResetContent(fmt.Sprintf("%s/reset-password/%s", m.serverName, passwordResetToken))
	return m.sendBasic(ctx, sendTo, c.Subject, c.Plain, c.Html)
}

// SendSubscriptionExpiryEmail notifies the user their subscription has expired.
func (m *Mailer) SendSubscriptionExpiryEmail(ctx context.Context, sendTo string) error {
	c := content.SubscriptionExpiryContent()
	return m.sendBasic(ctx, sendTo, c.Subject, c.Plain, c.Html)
}

// SendPendingDeleteEmail notifies the user their account is pending deletion.
func (m *Mailer) SendPendingDeleteEmail(ctx context.Context, sendTo string) error {
	c := content.PendingDeleteContent()
	return m.sendBasic(ctx, sendTo, c.Subject, c.Plain, c.Html)
}

// Verify performs basic syntax validation. Extend with more advanced service if needed.
func (m *Mailer) Verify(email string) error {
	if email == "" {
		return errors.New("email is empty")
	}
	if !m.emailRegex.MatchString(email) {
		return errors.New("invalid email format")
	}
	return nil
}

// sendBasic sends a plain/html email (subject, plain and html parts)
func (m *Mailer) sendBasic(ctx context.Context, toEmail, subject, plainContent, htmlContent string) error {
	from := mail.NewEmail("modDNS", "info@moddns.net")
	to := mail.NewEmail("User", toEmail)
	message := mail.NewSingleEmail(from, subject, to, plainContent, htmlContent)
	res, err := m.client.Send(message)
	log.Debug().Str("email", toEmail).Str("sendgrid_response", res.Body).Msg("SendGrid: request sent")
	if err != nil {
		return err
	}
	if res.StatusCode >= 400 {
		log.Error().Str("email", toEmail).Int("status", res.StatusCode).Msg("SendGrid: failed to send basic email")
		return ErrFailedToSendEmail
	}
	log.Info().Str("email", toEmail).Str("subject", subject).Msg("SendGrid: basic email sent")
	return nil
}
