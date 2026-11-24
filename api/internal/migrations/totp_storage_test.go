package migrations

import (
	"bytes"
	"context"

	"github.com/ivpn/dns/api/internal/totpcrypt"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
)

const plainSecret = "JBSWY3DPEHPK3PXPJBSWY3DPEHPK3PXP"

func (s *MigrateSuite) totpKeeper() *totpcrypt.Keeper {
	k, err := totpcrypt.New(bytes.Repeat([]byte{9}, totpcrypt.KeySize))
	s.Require().NoError(err)
	return k
}

func (s *MigrateSuite) accounts() *mongo.Collection {
	return s.client.Database(s.dbName).Collection(accountsCollection)
}

func (s *MigrateSuite) seedTOTP(totp bson.M) primitive.ObjectID {
	id := primitive.NewObjectID()
	_, err := s.accounts().InsertOne(context.Background(), bson.M{"_id": id, "email": id.Hex() + "@example.com", "mfa": bson.M{"totp": totp}})
	s.Require().NoError(err)
	return id
}

func (s *MigrateSuite) storedTOTP(id primitive.ObjectID) bson.M {
	var doc bson.M
	s.Require().NoError(s.accounts().FindOne(context.Background(), bson.M{"_id": id}).Decode(&doc))
	return doc["mfa"].(bson.M)["totp"].(bson.M)
}

func (s *MigrateSuite) resetAccounts() {
	_ = s.accounts().Drop(context.Background())
}

// specRef: api-endpoint-behaviour.md E9
func (s *MigrateSuite) TestTOTPStorageConvertsLegacyRecord() {
	s.resetAccounts()
	ctx := context.Background()
	k := s.totpKeeper()
	id := s.seedTOTP(bson.M{"enabled": true, "secret": plainSecret, "backup_codes": bson.A{"c1aaaaaaaaaaaaaa", "c2aaaaaaaaaaaaaa"}, "backup_codes_used": bson.A{"c0aaaaaaaaaaaaaa"}})

	stats, err := MigrateTOTPStorage(ctx, s.client, s.dbName, k)
	s.Require().NoError(err)
	s.Equal(1, stats.Migrated)
	s.Equal(0, stats.Failed)

	totp := s.storedTOTP(id)
	for _, f := range []string{"secret", "backup_codes", "backup_codes_used"} {
		s.NotContains(totp, f)
	}
	secret, err := k.OpenSecret(id.Hex(), totp["secret_sealed"].(string))
	s.Require().NoError(err)
	s.Equal(plainSecret, secret)
	s.ElementsMatch(bson.A{k.HashBackupCode(id.Hex(), "c1aaaaaaaaaaaaaa"), k.HashBackupCode(id.Hex(), "c2aaaaaaaaaaaaaa")}, totp["backup_code_hashes"])
	s.NotContains(totp, "backup_code_hashes_used", "used codes are dropped, not converted")
	s.Equal(true, totp["backup_codes_legacy"])
	s.Equal(true, totp["enabled"])
}

// A null used list converts like an empty one.
// specRef: api-endpoint-behaviour.md E9
func (s *MigrateSuite) TestTOTPStorageConvertsRecordWithNullUsedList() {
	s.resetAccounts()
	ctx := context.Background()
	k := s.totpKeeper()
	id := s.seedTOTP(bson.M{"enabled": true, "secret": plainSecret, "backup_codes": bson.A{"c1aaaaaaaaaaaaaa"}, "backup_codes_used": nil})

	stats, err := MigrateTOTPStorage(ctx, s.client, s.dbName, k)
	s.Require().NoError(err)
	s.Equal(1, stats.Migrated)
	s.Equal(0, stats.Failed)

	totp := s.storedTOTP(id)
	for _, f := range []string{"secret", "backup_codes", "backup_codes_used"} {
		s.NotContains(totp, f)
	}
	s.Equal(bson.A{k.HashBackupCode(id.Hex(), "c1aaaaaaaaaaaaaa")}, totp["backup_code_hashes"])
	s.Equal(true, totp["backup_codes_legacy"])
}

// A record whose codes were all used holds no code set to replace.
// specRef: api-endpoint-behaviour.md E9
func (s *MigrateSuite) TestTOTPStorageUsedOnlyCodesAreDropped() {
	s.resetAccounts()
	id := s.seedTOTP(bson.M{"enabled": true, "secret": plainSecret, "backup_codes": bson.A{}, "backup_codes_used": bson.A{"c0aaaaaaaaaaaaaa"}})

	stats, err := MigrateTOTPStorage(context.Background(), s.client, s.dbName, s.totpKeeper())
	s.Require().NoError(err)
	s.Equal(1, stats.Migrated)

	totp := s.storedTOTP(id)
	for _, f := range []string{"secret", "backup_codes", "backup_codes_used", "backup_code_hashes", "backup_codes_legacy"} {
		s.NotContains(totp, f)
	}
}

// A used-code digest list left by earlier builds is removed; nothing else changes.
// specRef: api-endpoint-behaviour.md E9
func (s *MigrateSuite) TestTOTPStorageRemovesUsedDigestList() {
	s.resetAccounts()
	record := bson.M{"enabled": true, "secret_sealed": "v1.kid.x", "backup_code_hashes": bson.A{"h1"}, "backup_code_hashes_used": bson.A{"h0"}}
	id := s.seedTOTP(record)

	stats, err := MigrateTOTPStorage(context.Background(), s.client, s.dbName, s.totpKeeper())
	s.Require().NoError(err)
	s.Equal(1, stats.Migrated)
	s.Equal(bson.M{"enabled": true, "secret_sealed": "v1.kid.x", "backup_code_hashes": bson.A{"h1"}}, s.storedTOTP(id))
}

// Only records holding TOTP material are rewritten; empty fields are left alone.
func (s *MigrateSuite) TestTOTPStorageSkipsRecordsWithoutMaterial() {
	s.resetAccounts()
	empty := bson.M{"enabled": false, "secret": "", "backup_codes": nil, "backup_codes_used": bson.A{}}
	id := s.seedTOTP(empty)

	stats, err := MigrateTOTPStorage(context.Background(), s.client, s.dbName, s.totpKeeper())
	s.Require().NoError(err)
	s.Equal(0, stats.Scanned)
	s.Equal(empty, s.storedTOTP(id))
}

// specRef: api-endpoint-behaviour.md E9
func (s *MigrateSuite) TestTOTPStorageIsIdempotent() {
	s.resetAccounts()
	ctx := context.Background()
	k := s.totpKeeper()
	id := s.seedTOTP(bson.M{"enabled": true, "secret": plainSecret, "backup_codes": bson.A{"c1aaaaaaaaaaaaaa"}})
	converted := s.seedTOTP(bson.M{"enabled": true, "secret_sealed": "v1.x.y", "backup_code_hashes": bson.A{"h1"}})

	_, err := MigrateTOTPStorage(ctx, s.client, s.dbName, k)
	s.Require().NoError(err)
	first := s.storedTOTP(id)

	stats, err := MigrateTOTPStorage(ctx, s.client, s.dbName, k)
	s.Require().NoError(err)
	s.Equal(0, stats.Scanned)
	s.Equal(first, s.storedTOTP(id))
	s.Equal(bson.M{"enabled": true, "secret_sealed": "v1.x.y", "backup_code_hashes": bson.A{"h1"}}, s.storedTOTP(converted))
}

// A backup code consumed between read and write must stay consumed.
// specRef: api-endpoint-behaviour.md E9
func (s *MigrateSuite) TestTOTPStorageNeverOverwritesConcurrentChange() {
	s.resetAccounts()
	ctx := context.Background()
	k := s.totpKeeper()
	id := s.seedTOTP(bson.M{"enabled": true, "secret": plainSecret, "backup_codes": bson.A{"c1aaaaaaaaaaaaaa", "c2aaaaaaaaaaaaaa"}, "backup_codes_used": bson.A{}})

	var stale bson.M
	s.Require().NoError(s.accounts().FindOne(ctx, bson.M{"_id": id}).Decode(&stale))
	_, err := s.accounts().UpdateByID(ctx, id, bson.M{
		"$pull": bson.M{"mfa.totp.backup_codes": "c1aaaaaaaaaaaaaa"},
	})
	s.Require().NoError(err)

	s.ErrorIs(convertTOTPRecord(ctx, s.accounts(), k, stale), errTOTPRecordChanged)
	s.Contains(s.storedTOTP(id), "backup_codes", "stale write must not land")

	_, err = MigrateTOTPStorage(ctx, s.client, s.dbName, k)
	s.Require().NoError(err)
	totp := s.storedTOTP(id)
	s.Equal(bson.A{k.HashBackupCode(id.Hex(), "c2aaaaaaaaaaaaaa")}, totp["backup_code_hashes"])
	s.NotContains(totp, "backup_code_hashes_used")
}

func (s *MigrateSuite) TestTOTPStorageRequiresKey() {
	_, err := MigrateTOTPStorage(context.Background(), s.client, s.dbName, nil)
	s.Error(err)
}

// specRef: api-endpoint-behaviour.md E9
func (s *MigrateSuite) TestTOTPStorageConvertsCodesWithoutSecret() {
	s.resetAccounts()
	k := s.totpKeeper()
	id := s.seedTOTP(bson.M{"enabled": true, "secret": "", "backup_codes": bson.A{"c1aaaaaaaaaaaaaa"}, "backup_codes_used": nil})

	stats, err := MigrateTOTPStorage(context.Background(), s.client, s.dbName, k)
	s.Require().NoError(err)
	s.Equal(1, stats.Migrated)

	totp := s.storedTOTP(id)
	s.NotContains(totp, "secret_sealed")
	s.NotContains(totp, "secret")
	s.Equal(bson.A{k.HashBackupCode(id.Hex(), "c1aaaaaaaaaaaaaa")}, totp["backup_code_hashes"])
}

// An enabled record without any material is left for the fail-closed read path.
func (s *MigrateSuite) TestTOTPStorageLeavesEnabledRecordWithoutMaterial() {
	s.resetAccounts()
	record := bson.M{"enabled": true, "secret": "", "backup_codes": nil, "backup_codes_used": nil}
	id := s.seedTOTP(record)

	stats, err := MigrateTOTPStorage(context.Background(), s.client, s.dbName, s.totpKeeper())
	s.Require().NoError(err)
	s.Equal(0, stats.Scanned)
	s.Equal(record, s.storedTOTP(id))
}
