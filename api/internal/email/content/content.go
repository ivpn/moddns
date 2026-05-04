package content

import "fmt"

const SupportEmail = "moddns@ivpn.net"

// EmailContent holds subject, plain text, and HTML for an email.
type EmailContent struct {
	Subject string
	Plain   string
	Html    string
}

// WelcomeContent returns the welcome email content.
func WelcomeContent(homeURL, verifyURL string) EmailContent {
	return EmailContent{
		Subject: "Welcome to modDNS",
		Plain:   fmt.Sprintf("Hello,\n\nWelcome to modDNS. Get started with using the service here: %s \n\nWarning: your email is not verified. Account recovery and critical service notification emails are disabled for unverified addresses. Follow this link to verify your email in modDNS settings: %s\n\nSent by modDNS", homeURL, verifyURL),
		Html:    fmt.Sprintf("<p>Hello,</p><p>Welcome to modDNS. Get started with using the service here: <a href=\"%s\">%s</a></p><p><strong>Warning:</strong> your email is not verified. Account recovery and critical service notification emails are disabled for unverified addresses. Follow this link to verify your email in modDNS settings: <a href=\"%s\">%s</a></p><p>Sent by modDNS</p>", homeURL, homeURL, verifyURL, verifyURL),
	}
}

// PasswordResetContent returns the password reset email content.
func PasswordResetContent(resetLink string) EmailContent {
	return EmailContent{
		Subject: "Reset your modDNS password",
		Plain:   fmt.Sprintf("Hello,\n\nYou have requested a password reset for your modDNS account.\n\nFollow this link to reset your password: %s\n\nThe URL is live for 60 minutes after generation.\n\nIf you did not request the password reset, please ignore this message or contact support at %s.\n\nRegards,\nmodDNS team", resetLink, SupportEmail),
		Html:    fmt.Sprintf("<p>Hello,</p><p>You have requested a password reset for your modDNS account.</p><p>Follow this link to reset your password: <a href=\"%s\">%s</a></p><p>The URL is live for 60 minutes after generation.</p><p>If you did not request the password reset, please ignore this message or contact support at <a href=\"mailto:%s\">%s</a>.</p><p>Regards,<br>modDNS team</p>", resetLink, resetLink, SupportEmail, SupportEmail),
	}
}

// SubscriptionExpiryContent returns the subscription expiry notification email content.
func SubscriptionExpiryContent() EmailContent {
	return EmailContent{
		Subject: "Limited Access Mode",
		Plain:   "Hello,\n\nYour modDNS account is in limited access mode.\n\nTo regain full access with no restrictions, add time to your IVPN account.\n\nSent by modDNS",
		Html:    "<p>Hello,</p><p>Your modDNS account is in limited access mode.</p><p>To regain full access with no restrictions, add time to your IVPN account.</p><p>Sent by modDNS</p>",
	}
}

// PendingDeleteContent returns the pending deletion notification email content.
func PendingDeleteContent() EmailContent {
	return EmailContent{
		Subject: "Your modDNS account is pending deletion",
		Plain:   "Hello,\n\nYour modDNS account has been in limited access mode for 14 days and is now pending deletion. DNS resolution has been disabled for your profiles.\n\nTo reinstate full access, add time to your IVPN account: https://www.ivpn.net\n\nRegards,\nmodDNS Staff",
		Html:    "<p>Hello,</p><p>Your modDNS account has been in limited access mode for 14 days and is now pending deletion. DNS resolution has been disabled for your profiles.</p><p>To reinstate full access, add time to your IVPN account: <a href=\"https://www.ivpn.net\">https://www.ivpn.net</a></p><p>Regards,<br>modDNS Staff</p>",
	}
}

// EmailVerificationOTPContent returns the email verification OTP content.
func EmailVerificationOTPContent(otp string) EmailContent {
	return EmailContent{
		Subject: "modDNS Email address verification",
		Plain:   fmt.Sprintf("Hello,\n\nHere is a one-time code to verify your modDNS registered email address: %s  \n\nIt expires in 15 minutes.\n\nNote: Unverified recipients will not receive account recovery emails.\n\nSent by modDNS", otp),
		Html:    fmt.Sprintf("<p>Hello,</p><p>Here is a one-time code to verify your modDNS registered email address: <strong>%s</strong></p><p>It expires in 15 minutes.</p><p><em>Note: Unverified recipients will not receive account recovery emails.</em></p><p>Sent by modDNS</p>", otp),
	}
}
