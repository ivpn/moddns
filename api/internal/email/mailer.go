package email

import (
	"context"
	"errors"

	"github.com/ivpn/dns/api/internal/email/mailpit"
	"github.com/ivpn/dns/api/internal/email/mailtrap"
	"github.com/ivpn/dns/api/internal/email/sendgrid"
)

const (
	MailerMailtrap = "mailtrap"
	MailerMailpit  = "mailpit"
	MailerSendgrid = "sendgrid"
)

// Mailer is an interface for sending emails
type Mailer interface {
	SendWelcomeEmail(ctx context.Context, sendTo, confirmationToken string) error
	SendPasswordResetEmail(ctx context.Context, sendTo, passwordResetToken string) error
	SendEmailVerificationOTP(ctx context.Context, sendTo, otp string) error
	SendSubscriptionExpiryEmail(ctx context.Context, sendTo string) error
	Verify(email string) error
}

// NewMailer creates a new Sender instance
// NewMailer creates a new Sender instance. For sendgrid implementation inboxId used as welcome template id and authToken as API key.
// NOTE: To avoid breaking signature, we overload parameters for sendgrid: inboxId -> welcome template, authToken -> API key.
// Future improvement: introduce structured config passed in.
func NewMailer(serverName, senderType, inboxId, authToken string) (Mailer, error) {
	switch senderType {
	case MailerMailtrap:
		return mailtrap.NewMailtrap(serverName, inboxId, authToken), nil
	case MailerMailpit:
		return mailpit.NewMailpit(serverName), nil
	case MailerSendgrid:
		return sendgrid.New(serverName, authToken), nil
	default:
		return nil, errors.New("unknown sender type")
	}
}
