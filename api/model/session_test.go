package model

import (
	"encoding/base64"
	"encoding/json"
	"reflect"
	"testing"
	"time"
)

func TestGenSessionToken(t *testing.T) {
	token, err := GenSessionToken()
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if len(token) == 0 {
		t.Fatalf("expected a token, got an empty string")
	}

	decoded, err := base64.StdEncoding.DecodeString(token)
	if err != nil {
		t.Fatalf("expected a valid base64 token, got error: %v", err)
	}

	if len(decoded) != 32 {
		t.Fatalf("expected token length of 32 bytes, got %d bytes", len(decoded))
	}
}

// Stored session data is JSON with the keys challenge, user_id and expires, plus
// fields this payload ignores; it must decode, and writes must keep those keys.
// specRef: A5, F9
func TestUnmarshalSessionData(t *testing.T) {
	tests := []struct {
		name      string
		data      string
		challenge string
	}{
		{
			name: "login session",
			data: `{"challenge":"","rpId":"","user_id":"dXNlcklE","expires":"2026-10-27T12:00:00Z","userVerification":""}`,
		},
		{
			name:      "ceremony state",
			data:      `{"challenge":"c2FsdA","rpId":"test.local","user_id":"dXNlcklE","expires":"2026-10-27T12:00:00Z","userVerification":"required","allowed_credentials":["AQ=="]}`,
			challenge: "c2FsdA",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			session := &Session{Data: []byte(tt.data)}
			if err := session.UnmarshalSessionData(); err != nil {
				t.Fatalf("expected no error, got %v", err)
			}
			want := SessionPayload{Challenge: tt.challenge, UserID: []byte("userID"), Expires: time.Date(2026, 10, 27, 12, 0, 0, 0, time.UTC)}
			if !reflect.DeepEqual(session.SessionData, want) {
				t.Fatalf("expected %+v, got %+v", want, session.SessionData)
			}
		})
	}

	data, err := json.Marshal(SessionPayload{UserID: []byte("userID"), Expires: time.Date(2026, 10, 27, 12, 0, 0, 0, time.UTC)})
	if err != nil {
		t.Fatalf("failed to marshal session payload: %v", err)
	}
	if want := `{"challenge":"","user_id":"dXNlcklE","expires":"2026-10-27T12:00:00Z"}`; string(data) != want {
		t.Fatalf("expected %s, got %s", want, data)
	}
}

func TestUnmarshalSessionData_InvalidData(t *testing.T) {
	session := &Session{
		Data: []byte("invalid data"),
	}

	err := session.UnmarshalSessionData()
	if err == nil {
		t.Fatalf("expected an error, got nil")
	}
}
