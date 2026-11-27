package mongodb

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"

	dberrors "github.com/ivpn/dns/api/db/errors"
	"github.com/ivpn/dns/api/model"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

func (s *AccountRepositorySuite) newTOTPAccount() string {
	s.T().Helper()
	accountID := primitive.NewObjectID().Hex()
	_, err := s.repo.CreateAccount(context.Background(), accountID+"@example.com", "password123", accountID, "profile-"+accountID)
	s.Require().NoError(err)
	return accountID
}

func (s *AccountRepositorySuite) rawTOTP(accountID string) bson.M {
	s.T().Helper()
	id, err := primitive.ObjectIDFromHex(accountID)
	s.Require().NoError(err)
	var doc bson.M
	s.Require().NoError(s.client.Database(s.dbName).Collection("accounts_test").FindOne(context.Background(), bson.M{"_id": id}).Decode(&doc))
	return doc["mfa"].(bson.M)["totp"].(bson.M)
}

func (s *AccountRepositorySuite) setRawTOTP(accountID string, totp bson.M) {
	s.T().Helper()
	id, err := primitive.ObjectIDFromHex(accountID)
	s.Require().NoError(err)
	_, err = s.client.Database(s.dbName).Collection("accounts_test").UpdateByID(context.Background(), id, bson.M{"$set": bson.M{"mfa.totp": totp}})
	s.Require().NoError(err)
}

// specRef: api-endpoint-behaviour.md E4, E5
func (s *AccountRepositorySuite) TestEnableTOTPStoresOnlyProtectedMaterial() {
	ctx := context.Background()
	accountID := s.newTOTPAccount()

	s.Require().NoError(s.repo.EnableTOTP(ctx, accountID, "v1.kid.sealed", []string{"h1", "h2"}))

	totp := s.rawTOTP(accountID)
	s.Equal(true, totp["enabled"])
	s.Equal("v1.kid.sealed", totp["secret_sealed"])
	s.ElementsMatch(bson.A{"h1", "h2"}, totp["backup_code_hashes"])
	for _, legacy := range []string{"secret", "backup_codes", "backup_codes_used", "backup_codes_legacy", "backup_code_hashes_used"} {
		s.NotContains(totp, legacy)
	}
}

// specRef: api-endpoint-behaviour.md E2
func (s *AccountRepositorySuite) TestEnableTOTPRefusesWhenAlreadyEnabled() {
	ctx := context.Background()
	accountID := s.newTOTPAccount()
	s.Require().NoError(s.repo.EnableTOTP(ctx, accountID, "v1.kid.first", []string{"h1"}))

	err := s.repo.EnableTOTP(ctx, accountID, "v1.kid.second", []string{"h2"})
	s.ErrorIs(err, dberrors.ErrTOTPStateChanged)
	s.Equal("v1.kid.first", s.rawTOTP(accountID)["secret_sealed"])
}

// specRef: api-endpoint-behaviour.md E7
func (s *AccountRepositorySuite) TestDisableTOTPRemovesAllMaterial() {
	ctx := context.Background()
	accountID := s.newTOTPAccount()
	s.setRawTOTP(accountID, bson.M{
		"enabled": true, "secret_sealed": "v1.kid.x", "backup_code_hashes": bson.A{"h1"},
		"backup_code_hashes_used": bson.A{"h0"}, "backup_codes_legacy": true,
		"secret": "LEGACY", "backup_codes": bson.A{"c1"}, "backup_codes_used": bson.A{"c0"},
	})

	s.Require().NoError(s.repo.DisableTOTP(ctx, accountID, model.TOTPProof{SecretSealed: "v1.kid.x", BackupCodeHash: "h1"}))

	s.Equal(bson.M{"enabled": false}, s.rawTOTP(accountID))
}

// The code that authorised turning 2FA off is checked in the same update, so
// a code used up meanwhile leaves 2FA on.
// specRef: api-endpoint-behaviour.md E3, E7
func (s *AccountRepositorySuite) TestDisableTOTPRequiresTheVerifiedState() {
	ctx := context.Background()
	accountID := s.newTOTPAccount()
	s.Require().NoError(s.repo.EnableTOTP(ctx, accountID, "v1.kid.x", []string{"h1", "h2"}))

	for name, proof := range map[string]model.TOTPProof{
		"used backup code":   {SecretSealed: "v1.kid.x", BackupCodeHash: "h9"},
		"secret replaced":    {SecretSealed: "v1.kid.old"},
		"no secret in proof": {},
	} {
		s.Error(s.repo.DisableTOTP(ctx, accountID, proof), name)
		s.Equal(true, s.rawTOTP(accountID)["enabled"], name)
	}
	s.ErrorIs(s.repo.DisableTOTP(ctx, accountID, model.TOTPProof{SecretSealed: "v1.kid.x", BackupCodeHash: "h9"}), dberrors.ErrTOTPStateChanged)

	s.Require().NoError(s.repo.DisableTOTP(ctx, accountID, model.TOTPProof{SecretSealed: "v1.kid.x"}))
	s.Equal(bson.M{"enabled": false}, s.rawTOTP(accountID))
	s.ErrorIs(s.repo.DisableTOTP(ctx, accountID, model.TOTPProof{SecretSealed: "v1.kid.x"}), dberrors.ErrTOTPStateChanged)
}

// specRef: api-endpoint-behaviour.md E13
func (s *AccountRepositorySuite) TestReplaceBackupCodesReplacesTheWholeSet() {
	ctx := context.Background()
	accountID := s.newTOTPAccount()
	s.setRawTOTP(accountID, bson.M{
		"enabled": true, "secret_sealed": "v1.kid.x", "backup_code_hashes": bson.A{"h1", "h2"},
		"backup_codes_legacy": true, "backup_code_hashes_used": bson.A{"h0"},
	})

	s.Require().NoError(s.repo.ReplaceBackupCodes(ctx, accountID, model.TOTPProof{SecretSealed: "v1.kid.x", BackupCodeHash: "h1"}, []string{"n1", "n2", "n3"}))

	s.Equal(bson.M{"enabled": true, "secret_sealed": "v1.kid.x", "backup_code_hashes": bson.A{"n1", "n2", "n3"}}, s.rawTOTP(accountID))
}

// specRef: api-endpoint-behaviour.md E13
func (s *AccountRepositorySuite) TestReplaceBackupCodesRequiresTheVerifiedState() {
	ctx := context.Background()
	accountID := s.newTOTPAccount()
	s.Require().NoError(s.repo.EnableTOTP(ctx, accountID, "v1.kid.x", []string{"h1"}))

	for name, proof := range map[string]model.TOTPProof{
		"used backup code":   {SecretSealed: "v1.kid.x", BackupCodeHash: "h9"},
		"secret replaced":    {SecretSealed: "v1.kid.old"},
		"no secret in proof": {},
	} {
		s.Error(s.repo.ReplaceBackupCodes(ctx, accountID, proof, []string{"n1"}), name)
	}
	s.ErrorIs(s.repo.ReplaceBackupCodes(ctx, accountID, model.TOTPProof{SecretSealed: "v1.kid.old"}, []string{"n1"}), dberrors.ErrTOTPStateChanged)
	s.Equal(bson.A{"h1"}, s.rawTOTP(accountID)["backup_code_hashes"])

	s.Require().NoError(s.repo.DisableTOTP(ctx, accountID, model.TOTPProof{SecretSealed: "v1.kid.x"}))
	s.ErrorIs(s.repo.ReplaceBackupCodes(ctx, accountID, model.TOTPProof{SecretSealed: "v1.kid.x"}, []string{"n1"}), dberrors.ErrTOTPStateChanged)
	s.Equal(bson.M{"enabled": false}, s.rawTOTP(accountID))
}

// Two requests authorised by the same backup code: only one replaces the set.
// specRef: api-endpoint-behaviour.md E13
func (s *AccountRepositorySuite) TestReplaceBackupCodesConcurrentUseOfOneCodeSucceedsOnce() {
	ctx := context.Background()
	accountID := s.newTOTPAccount()
	s.Require().NoError(s.repo.EnableTOTP(ctx, accountID, "v1.kid.x", []string{"h1", "h2"}))

	var wins atomic.Int32
	var wg sync.WaitGroup
	for i := range 20 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			err := s.repo.ReplaceBackupCodes(ctx, accountID, model.TOTPProof{SecretSealed: "v1.kid.x", BackupCodeHash: "h1"}, []string{fmt.Sprintf("n%d", i)})
			if err == nil {
				wins.Add(1)
				return
			}
			s.ErrorIs(err, dberrors.ErrTOTPStateChanged)
		}()
	}
	wg.Wait()
	s.Equal(int32(1), wins.Load())
}

// specRef: api-endpoint-behaviour.md E6
func (s *AccountRepositorySuite) TestConsumeBackupCodeIsSingleUse() {
	ctx := context.Background()
	accountID := s.newTOTPAccount()
	s.Require().NoError(s.repo.EnableTOTP(ctx, accountID, "v1.kid.x", []string{"h1", "h2"}))

	ok, err := s.repo.ConsumeBackupCode(ctx, accountID, "h1")
	s.Require().NoError(err)
	s.True(ok)

	ok, err = s.repo.ConsumeBackupCode(ctx, accountID, "h1")
	s.Require().NoError(err)
	s.False(ok)

	ok, err = s.repo.ConsumeBackupCode(ctx, accountID, "unknown")
	s.Require().NoError(err)
	s.False(ok)

	totp := s.rawTOTP(accountID)
	s.Equal(bson.A{"h2"}, totp["backup_code_hashes"])
	s.NotContains(totp, "backup_code_hashes_used", "used codes are not recorded")
}

// specRef: api-endpoint-behaviour.md E6
func (s *AccountRepositorySuite) TestConsumeBackupCodeConcurrentUseSucceedsOnce() {
	ctx := context.Background()
	accountID := s.newTOTPAccount()
	s.Require().NoError(s.repo.EnableTOTP(ctx, accountID, "v1.kid.x", []string{"h1"}))

	var wins atomic.Int32
	var wg sync.WaitGroup
	for range 20 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			ok, err := s.repo.ConsumeBackupCode(ctx, accountID, "h1")
			s.NoError(err)
			if ok {
				wins.Add(1)
			}
		}()
	}
	wg.Wait()
	s.Equal(int32(1), wins.Load())
}

func (s *AccountRepositorySuite) TestConsumeBackupCodeRequiresEnabledTOTP() {
	ctx := context.Background()
	accountID := s.newTOTPAccount()
	s.setRawTOTP(accountID, bson.M{"enabled": false, "backup_code_hashes": bson.A{"h1"}})

	ok, err := s.repo.ConsumeBackupCode(ctx, accountID, "h1")
	s.Require().NoError(err)
	s.False(ok)
}

// specRef: api-endpoint-behaviour.md E8
func (s *AccountRepositorySuite) TestConsumeLegacyBackupCode() {
	ctx := context.Background()
	accountID := s.newTOTPAccount()
	s.setRawTOTP(accountID, bson.M{"enabled": true, "secret": "LEGACY", "backup_codes": bson.A{"c1", "c2"}, "backup_codes_used": bson.A{}})

	var wins atomic.Int32
	var wg sync.WaitGroup
	for range 10 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			ok, err := s.repo.ConsumeLegacyBackupCode(ctx, accountID, "c1")
			s.NoError(err)
			if ok {
				wins.Add(1)
			}
		}()
	}
	wg.Wait()
	s.Equal(int32(1), wins.Load())

	totp := s.rawTOTP(accountID)
	s.Equal(bson.A{"c2"}, totp["backup_codes"])
	s.Equal(bson.A{}, totp["backup_codes_used"], "used codes are not recorded")
}

// Consuming a code never reads or writes a used-codes list, whatever shape a
// stored record has for it. specRef: api-endpoint-behaviour.md E6, E8
func (s *AccountRepositorySuite) TestConsumeBackupCodeIgnoresUsedList() {
	ctx := context.Background()
	shapes := []struct {
		name string
		used any
	}{
		{"null", nil},
		{"missing", "omit"},
		{"string", "c0"},
	}
	for _, shape := range shapes {
		s.Run(shape.name, func() {
			accountID := s.newTOTPAccount()
			totp := bson.M{"enabled": true, "secret": "LEGACY", "backup_codes": bson.A{"c1", "c2"}, "backup_code_hashes": bson.A{"h1"}}
			if shape.used != "omit" {
				totp["backup_codes_used"] = shape.used
			}
			s.setRawTOTP(accountID, totp)

			var wins atomic.Int32
			var wg sync.WaitGroup
			for range 10 {
				wg.Add(1)
				go func() {
					defer wg.Done()
					ok, err := s.repo.ConsumeLegacyBackupCode(ctx, accountID, "c1")
					s.NoError(err)
					if ok {
						wins.Add(1)
					}
				}()
			}
			wg.Wait()
			s.Equal(int32(1), wins.Load())

			ok, err := s.repo.ConsumeBackupCode(ctx, accountID, "h1")
			s.Require().NoError(err)
			s.True(ok)

			got := s.rawTOTP(accountID)
			s.Equal(bson.A{"c2"}, got["backup_codes"])
			s.Equal(bson.A{}, got["backup_code_hashes"])
			s.NotContains(got, "backup_code_hashes_used")
			if shape.used == "omit" {
				s.NotContains(got, "backup_codes_used")
			} else {
				s.Equal(shape.used, got["backup_codes_used"])
			}
		})
	}
}

// A code stored as a single string rather than in a list is not matched.
func (s *AccountRepositorySuite) TestConsumeBackupCodeIgnoresNonArrayUnusedList() {
	ctx := context.Background()
	accountID := s.newTOTPAccount()
	s.setRawTOTP(accountID, bson.M{"enabled": true, "secret": "LEGACY", "backup_codes": "c1"})

	ok, err := s.repo.ConsumeLegacyBackupCode(ctx, accountID, "c1")
	s.Require().NoError(err)
	s.False(ok)
}

// Account updates load, modify and write the whole document; they must not
// write back a stale copy of the 2FA state. specRef: api-endpoint-behaviour.md E11
func (s *AccountRepositorySuite) TestUpdateAccountNeverWritesMFA() {
	ctx := context.Background()
	accountID := s.newTOTPAccount()
	s.Require().NoError(s.repo.EnableTOTP(ctx, accountID, "v1.kid.x", []string{"h1", "h2"}))

	stale, err := s.repo.GetAccountById(ctx, accountID)
	s.Require().NoError(err)

	ok, err := s.repo.ConsumeBackupCode(ctx, accountID, "h1")
	s.Require().NoError(err)
	s.Require().True(ok)

	stale.ErrorReportsConsent = true
	_, err = s.repo.UpdateAccount(ctx, stale)
	s.Require().NoError(err)

	fresh, err := s.repo.GetAccountById(ctx, accountID)
	s.Require().NoError(err)
	s.True(fresh.ErrorReportsConsent, "non-MFA fields are still updated")
	s.Equal([]string{"h2"}, fresh.MFA.TOTP.BackupCodeHashes, "consumed code must not come back")
}
