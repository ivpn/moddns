package migrations

import (
	"context"
	"errors"
	"fmt"

	"github.com/ivpn/dns/api/internal/totpcrypt"
	"github.com/rs/zerolog/log"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

const accountsCollection = "accounts"

// totpConvertAttempts bounds retries when a record changes between read and write.
const totpConvertAttempts = 3

var errTOTPRecordChanged = errors.New("totp record changed during conversion")

// Records holding plaintext material (a non-empty secret or code list) or a
// used-code digest list. Empty legacy fields carry nothing and are left as they are.
var legacyTOTPFilter = bson.D{{Key: "$or", Value: bson.A{
	bson.D{{Key: "mfa.totp.secret", Value: bson.D{{Key: "$nin", Value: bson.A{"", nil}}}}},
	bson.D{{Key: "mfa.totp.backup_codes.0", Value: bson.D{{Key: "$exists", Value: true}}}},
	bson.D{{Key: "mfa.totp.backup_codes_used.0", Value: bson.D{{Key: "$exists", Value: true}}}},
	bson.D{{Key: "mfa.totp.backup_code_hashes_used", Value: bson.D{{Key: "$exists", Value: true}}}},
}}}

// MigrateTOTPStorage converts every account still holding plaintext TOTP
// fields: the secret is sealed, unused backup codes become keyed digests and
// the account is flagged backup_codes_legacy, used codes (plaintext or digest)
// are dropped. Plaintext fields are removed.
//
// Each document is rewritten with a filter on the exact values read, so a
// concurrent change (a backup code consumed meanwhile) is never overwritten;
// the document is re-read and converted again. Idempotent.
//
// Run only when every API instance reads both formats: an instance without
// that support cannot verify converted records.
func MigrateTOTPStorage(ctx context.Context, client *mongo.Client, dbName string, keeper *totpcrypt.Keeper) (Stats, error) {
	if keeper == nil {
		return Stats{}, errors.New("totp storage migration: no storage key")
	}
	coll := client.Database(dbName).Collection(accountsCollection)

	cursor, err := coll.Find(ctx, legacyTOTPFilter, options.Find().SetProjection(bson.D{{Key: "mfa.totp", Value: 1}}))
	if err != nil {
		return Stats{}, fmt.Errorf("find legacy totp records: %w", err)
	}
	defer cursor.Close(ctx)

	var stats Stats
	for cursor.Next(ctx) {
		stats.Scanned++
		var doc bson.M
		if err := cursor.Decode(&doc); err != nil {
			log.Error().Err(err).Msg("totp storage migration: decode failed")
			stats.Failed++
			continue
		}
		id, _ := doc["_id"].(primitive.ObjectID)

		for attempt := 1; ; attempt++ {
			err = convertTOTPRecord(ctx, coll, keeper, doc)
			if !errors.Is(err, errTOTPRecordChanged) || attempt == totpConvertAttempts {
				break
			}
			doc = bson.M{}
			if err = coll.FindOne(ctx, bson.D{{Key: "_id", Value: id}}, options.FindOne().SetProjection(bson.D{{Key: "mfa.totp", Value: 1}})).Decode(&doc); err != nil {
				break
			}
		}
		switch {
		case errors.Is(err, mongo.ErrNoDocuments):
			stats.Skipped++ // account deleted meanwhile
		case err != nil:
			log.Error().Err(err).Str("account_id", id.Hex()).Msg("totp storage migration: conversion failed")
			stats.Failed++
		default:
			stats.Migrated++
		}
	}
	if err := cursor.Err(); err != nil {
		return stats, fmt.Errorf("cursor iteration: %w", err)
	}
	return stats, nil
}

// convertTOTPRecord rewrites one account read as doc. Returns
// errTOTPRecordChanged when the stored record no longer matches doc.
func convertTOTPRecord(ctx context.Context, coll *mongo.Collection, keeper *totpcrypt.Keeper, doc bson.M) error {
	id, ok := doc["_id"].(primitive.ObjectID)
	if !ok {
		return errors.New("account _id is not an ObjectID")
	}
	accountID := id.Hex()
	totp := nestedTOTP(doc)

	filter := bson.D{{Key: "_id", Value: id}}
	for _, f := range []string{"secret", "backup_codes", "backup_codes_used"} {
		v, present := totp[f]
		if present {
			filter = append(filter, bson.E{Key: "mfa.totp." + f, Value: v})
		} else {
			filter = append(filter, bson.E{Key: "mfa.totp." + f, Value: bson.D{{Key: "$exists", Value: false}}})
		}
	}

	set := bson.D{}
	if secret, _ := totp["secret"].(string); secret != "" {
		if _, hasSealed := totp["secret_sealed"]; !hasSealed {
			sealed, err := keeper.SealSecret(accountID, secret)
			if err != nil {
				return err
			}
			set = append(set, bson.E{Key: "mfa.totp.secret_sealed", Value: sealed})
		}
	}
	// Used codes are dropped, not converted: a used code is never accepted again.
	unused := hashAll(keeper, accountID, stringSlice(totp["backup_codes"]))
	if len(unused) > 0 {
		set = append(set, bson.E{Key: "mfa.totp.backup_codes_legacy", Value: true})
	}

	update := bson.D{{Key: "$unset", Value: bson.D{
		{Key: "mfa.totp.secret", Value: ""},
		{Key: "mfa.totp.backup_codes", Value: ""},
		{Key: "mfa.totp.backup_codes_used", Value: ""},
		{Key: "mfa.totp.backup_code_hashes_used", Value: ""},
	}}}
	if len(set) > 0 {
		update = append(update, bson.E{Key: "$set", Value: set})
	}
	addToSet := bson.D{}
	if len(unused) > 0 {
		addToSet = append(addToSet, bson.E{Key: "mfa.totp.backup_code_hashes", Value: bson.D{{Key: "$each", Value: unused}}})
	}
	if len(addToSet) > 0 {
		update = append(update, bson.E{Key: "$addToSet", Value: addToSet})
	}

	res, err := coll.UpdateOne(ctx, filter, update)
	if err != nil {
		return err
	}
	if res.MatchedCount == 0 {
		return errTOTPRecordChanged
	}
	return nil
}

func nestedTOTP(doc bson.M) bson.M {
	mfa, _ := doc["mfa"].(bson.M)
	totp, _ := mfa["totp"].(bson.M)
	if totp == nil {
		return bson.M{}
	}
	return totp
}

func stringSlice(v any) []string {
	arr, _ := v.(bson.A)
	out := make([]string, 0, len(arr))
	for _, e := range arr {
		if s, ok := e.(string); ok && s != "" {
			out = append(out, s)
		}
	}
	return out
}

func hashAll(keeper *totpcrypt.Keeper, accountID string, codes []string) bson.A {
	out := make(bson.A, 0, len(codes))
	for _, c := range codes {
		out = append(out, keeper.HashBackupCode(accountID, c))
	}
	return out
}
