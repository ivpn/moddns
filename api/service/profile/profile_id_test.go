package profile

import (
	"context"
	"errors"
	"testing"

	dbErrors "github.com/ivpn/dns/api/db/errors"
	"github.com/ivpn/dns/api/db/repository"
	"github.com/ivpn/dns/api/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type sequenceGen struct {
	ids []string
	n   int
}

func (g *sequenceGen) Generate() (string, error) {
	id := g.ids[g.n%len(g.ids)]
	g.n++
	return id, nil
}

// uniqueIdRepo behaves like the profile_id unique index.
type uniqueIdRepo struct {
	repository.ProfileRepository
	taken    map[string]bool
	inserted []*model.Profile
	inserts  int
	err      error
}

func (r *uniqueIdRepo) CreateProfile(_ context.Context, profile *model.Profile) error {
	r.inserts++
	if r.err != nil {
		return r.err
	}
	if r.taken[profile.ProfileId] {
		return dbErrors.ErrProfileIdAlreadyExists
	}
	copied := *profile
	r.inserted = append(r.inserted, &copied)
	return nil
}

func newProfileWithSettings() *model.Profile {
	profile := model.NewProfile("", "Profile", "account1")
	profile.Settings = model.NewSettings()
	return profile
}

// specRef: registration-subscription-behaviour.md #R12
func TestInsertWithNewProfileId_RedrawsTakenId(t *testing.T) {
	repo := &uniqueIdRepo{taken: map[string]bool{"aaaaaaaaaa": true}}
	svc := &ProfileService{IdGen: &sequenceGen{ids: []string{"aaaaaaaaaa", "bbbbbbbbbb"}}, ProfileRepository: repo}
	profile := newProfileWithSettings()

	err := svc.insertWithNewProfileId(context.Background(), profile)

	require.NoError(t, err)
	assert.Equal(t, "bbbbbbbbbb", profile.ProfileId)
	assert.Equal(t, "bbbbbbbbbb", profile.Settings.ProfileId)
	require.Len(t, repo.inserted, 1)
	assert.Equal(t, "bbbbbbbbbb", repo.inserted[0].ProfileId)
}

// specRef: registration-subscription-behaviour.md #R12
func TestInsertWithNewProfileId_OtherInsertErrorIsReturned(t *testing.T) {
	insertErr := errors.New("store down")
	repo := &uniqueIdRepo{err: insertErr}
	svc := &ProfileService{IdGen: &sequenceGen{ids: []string{"aaaaaaaaaa"}}, ProfileRepository: repo}

	err := svc.insertWithNewProfileId(context.Background(), newProfileWithSettings())

	require.ErrorIs(t, err, insertErr)
	assert.Equal(t, 1, repo.inserts, "only a taken ID is redrawn")
}

// specRef: registration-subscription-behaviour.md #R12
func TestInsertWithNewProfileId_GivesUpAfterBoundedAttempts(t *testing.T) {
	repo := &uniqueIdRepo{taken: map[string]bool{"aaaaaaaaaa": true}}
	svc := &ProfileService{IdGen: &sequenceGen{ids: []string{"aaaaaaaaaa"}}, ProfileRepository: repo}

	err := svc.insertWithNewProfileId(context.Background(), newProfileWithSettings())

	require.ErrorIs(t, err, ErrProfileIdUnavailable)
	assert.Equal(t, profileIdAttempts, repo.inserts)
}
