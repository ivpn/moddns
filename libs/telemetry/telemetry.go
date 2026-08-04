// Package telemetry centralizes Sentry client construction for all services.
// Every Sentry client in the project must be built from InitOptions or
// LogWriterConfig so that delivery-time scrubbing (logging.SensitiveLogKeys,
// email redaction, request sanitization) is always installed.
//
// Compatibility: services select their own sentry-go version via MVS (api
// pins v0.31.1, proxy v0.36.0), so this package must stay on the API surface
// that is stable across that range.
package telemetry

import (
	"time"

	"github.com/getsentry/sentry-go"
	sentryzerolog "github.com/getsentry/sentry-go/zerolog"
	"github.com/rs/zerolog"

	"github.com/ivpn/dns/libs/logging"
)

// Config carries the service's Sentry settings.
type Config struct {
	DSN         string
	Environment string
	Release     string
}

// credentialHeaders are request headers that carry credentials, client
// network identity or device information and must not leave the host.
var credentialHeaders = []string{"Authorization", "Cookie", "X-Forwarded-For", "X-Real-Ip", "User-Agent"}

// InitOptions returns the ClientOptions for a service's global Sentry hub
// (panic recovery, HTTP middleware) with scrubbing hooks installed.
func InitOptions(cfg Config) sentry.ClientOptions {
	return sentry.ClientOptions{
		Dsn:                   cfg.DSN,
		Environment:           cfg.Environment,
		Release:               cfg.Release,
		TracesSampleRate:      1.0,
		AttachStacktrace:      true,
		EnableTracing:         true,
		BeforeSend:            Scrub,
		BeforeSendTransaction: Scrub,
		BeforeBreadcrumb:      ScrubBreadcrumb,
	}
}

// LogWriterConfig returns the configuration for the zerolog->Sentry writer.
// Only Error and above become Sentry events; breadcrumbs are disabled because
// the writer's hub is process-wide and would interleave fields from unrelated
// requests.
func LogWriterConfig(cfg Config) sentryzerolog.Config {
	return sentryzerolog.Config{
		ClientOptions: sentry.ClientOptions{
			Dsn:                   cfg.DSN,
			Environment:           cfg.Environment,
			Release:               cfg.Release,
			BeforeSend:            Scrub,
			BeforeSendTransaction: Scrub,
			BeforeBreadcrumb:      ScrubBreadcrumb,
		},
		Options: sentryzerolog.Options{
			Levels:          []zerolog.Level{zerolog.ErrorLevel, zerolog.FatalLevel, zerolog.PanicLevel},
			WithBreadcrumbs: false,
			FlushTimeout:    3 * time.Second,
		},
	}
}

// Scrub removes sensitive material from an event before delivery: structured
// log fields listed in logging.SensitiveLogKeys, email addresses embedded in
// message and exception text, and request payloads/credential headers.
// Registered as BeforeSend and BeforeSendTransaction on every client.
func Scrub(event *sentry.Event, _ *sentry.EventHint) *sentry.Event {
	if event == nil {
		return nil
	}

	logging.ScrubFields(event.Extra)
	for key, value := range event.Extra {
		if s, ok := value.(string); ok {
			event.Extra[key] = logging.RedactEmails(s)
		}
	}

	event.Message = logging.RedactEmails(event.Message)
	for i := range event.Exception {
		event.Exception[i].Value = logging.RedactEmails(event.Exception[i].Value)
	}

	if event.Request != nil {
		event.Request.Data = ""
		event.Request.Cookies = ""
		event.Request.QueryString = ""
		for _, header := range credentialHeaders {
			delete(event.Request.Headers, header)
		}
	}

	return event
}

// ScrubBreadcrumb applies the same field and email scrubbing to breadcrumbs.
// Registered as BeforeBreadcrumb on every client.
func ScrubBreadcrumb(breadcrumb *sentry.Breadcrumb, _ *sentry.BreadcrumbHint) *sentry.Breadcrumb {
	if breadcrumb == nil {
		return nil
	}
	logging.ScrubFields(breadcrumb.Data)
	breadcrumb.Message = logging.RedactEmails(breadcrumb.Message)
	return breadcrumb
}
