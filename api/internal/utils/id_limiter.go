package utils

import (
	"context"
	"strconv"
	"time"

	"github.com/rs/zerolog/log"
)

type Cache interface {
	Get(context.Context, string) (string, error)
	Incr(context.Context, string, time.Duration) (int64, error)
	Del(context.Context, string) error
}

type IDLimiter struct {
	ID    string
	Label string
	Max   int
	Exp   time.Duration
	Cache Cache
}

func (l *IDLimiter) Tick() error {
	key := l.Label + ":" + l.ID
	_, err := l.Cache.Incr(context.Background(), key, l.Exp)
	if err != nil {
		log.Error().Err(err).Msg("error incrementing failed attempts")
		return err
	}
	return nil
}

func (l *IDLimiter) IsAllowed() bool {
	failedAttempts, err := l.Cache.Get(context.Background(), l.Label+":"+l.ID)
	if err != nil {
		failedAttempts = "0"
	}
	failedAttemptsInt, err := strconv.Atoi(failedAttempts)
	if err != nil {
		failedAttemptsInt = 0
	}
	if failedAttemptsInt > l.Max {
		return false
	}

	return true
}

// Reset clears the failure count.
func (l *IDLimiter) Reset() error {
	if err := l.Cache.Del(context.Background(), l.Label+":"+l.ID); err != nil {
		log.Error().Err(err).Msg("error resetting failed attempts")
		return err
	}
	return nil
}

// Attempt counts one attempt before it is made and reports whether it is
// within Max for the current window. Counting first bounds concurrent
// attempts; a counter error refuses the attempt.
func (l *IDLimiter) Attempt() (bool, error) {
	n, err := l.Cache.Incr(context.Background(), l.Label+":"+l.ID, l.Exp)
	if err != nil {
		log.Error().Err(err).Msg("error counting attempt")
		return false, err
	}
	return n <= int64(l.Max), nil
}
