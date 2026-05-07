// Package migrations provides Go-based data migrations that run on API
// startup when enabled via config flags. Unlike the JSON schema migrations
// in api/db/mongodb/migrations/ (managed by golang-migrate), these are
// custom Go logic for data transformations that can't be expressed in JSON.
package migrations

import (
	"context"
	"fmt"

	"github.com/rs/zerolog/log"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
)

// subscriptionsCollection is the collection name used by api/db/mongodb.
const subscriptionsCollection = "subscriptions"

// Stats captures the outcome of a migration run.
type Stats struct {
	Scanned  int // total documents the cursor yielded
	Migrated int // documents whose _id was rewritten from !=0x04 to 0x04
	Skipped  int // already-0x04 docs, or docs with unexpected _id shape
	Failed   int // per-document errors (do not abort the run)
}

// MigrateSubscriptionUUIDSubtype walks the subscriptions collection and
// rewrites every _id with a non-0x04 binary subtype to subtype 0x04.
//
// Background: the mongo-driver v1 default codec encodes uuid.UUID as BSON
// binary subtype 0x00 (generic). A registered codec in libs/store/mongodb.go
// now writes subtype 0x04 going forward, but existing documents in staging
// and production still carry subtype 0x00 and therefore stop matching
// MarkNotified queries sent by the new code.
//
// The migration is idempotent: re-running it on a fully-migrated collection
// scans every doc, sees subtype 0x04, increments Skipped, and returns with
// Migrated == 0.
func MigrateSubscriptionUUIDSubtype(ctx context.Context, client *mongo.Client, dbName string) (Stats, error) {
	coll := client.Database(dbName).Collection(subscriptionsCollection)

	cursor, err := coll.Find(ctx, bson.D{})
	if err != nil {
		return Stats{}, fmt.Errorf("find subscriptions: %w", err)
	}
	defer cursor.Close(ctx)

	var stats Stats
	for cursor.Next(ctx) {
		stats.Scanned++

		var raw bson.Raw
		if err := cursor.Decode(&raw); err != nil {
			log.Error().Err(err).Int("scanned", stats.Scanned).Msg("decode raw subscription failed")
			stats.Failed++
			continue
		}

		subtype, data, ok := raw.Lookup("_id").BinaryOK()
		if !ok {
			log.Warn().Int("scanned", stats.Scanned).Msg("subscription _id is not binary, skipping")
			stats.Skipped++
			continue
		}
		if subtype == 0x04 {
			stats.Skipped++ // already migrated
			continue
		}
		if subtype != 0x00 && subtype != 0x03 {
			log.Warn().Uint8("subtype", subtype).Msg("unexpected _id subtype, skipping")
			stats.Skipped++
			continue
		}

		if err := rewriteIDSubtype(ctx, coll, raw, subtype, data); err != nil {
			log.Error().Err(err).Msg("rewrite _id subtype failed for document")
			stats.Failed++
			continue
		}
		stats.Migrated++
	}
	if err := cursor.Err(); err != nil {
		return stats, fmt.Errorf("cursor iteration: %w", err)
	}

	return stats, nil
}

// rewriteIDSubtype rebuilds a document with its _id recoded as
// primitive.Binary subtype 0x04, then inserts the rewritten copy and deletes
// the legacy row. The two writes are NOT wrapped in a multi-document
// transaction so the migration runs against standalone and replica-set
// deployments alike.
//
// Safety comes from fixed ordering (insert-new, delete-old) plus an
// idempotency pre-check: if a previous interrupted run left both copies
// coexisting, the next run detects the 0x04 copy, skips the insert, and
// finishes by deleting the legacy row.
func rewriteIDSubtype(
	ctx context.Context,
	coll *mongo.Collection,
	raw bson.Raw,
	oldSubtype byte,
	idBytes []byte,
) error {
	var doc bson.D
	if err := bson.Unmarshal(raw, &doc); err != nil {
		return fmt.Errorf("unmarshal document: %w", err)
	}

	newID := primitive.Binary{Subtype: 0x04, Data: idBytes}
	oldID := primitive.Binary{Subtype: oldSubtype, Data: idBytes}

	for i := range doc {
		if doc[i].Key == "_id" {
			doc[i].Value = newID
			break
		}
	}

	// Idempotency pre-check: if the 0x04 copy is already present from a
	// previous interrupted run, just finish the legacy delete.
	err := coll.FindOne(ctx, bson.D{{Key: "_id", Value: newID}}).Err()
	if err == nil {
		if _, derr := coll.DeleteOne(ctx, bson.D{{Key: "_id", Value: oldID}}); derr != nil {
			return fmt.Errorf("delete legacy _id after partial rerun: %w", derr)
		}
		return nil
	}
	if err != mongo.ErrNoDocuments {
		return fmt.Errorf("check for existing 0x04 _id: %w", err)
	}

	if _, err := coll.InsertOne(ctx, doc); err != nil {
		return fmt.Errorf("insert rewritten document: %w", err)
	}
	if _, err := coll.DeleteOne(ctx, bson.D{{Key: "_id", Value: oldID}}); err != nil {
		return fmt.Errorf("delete legacy _id: %w", err)
	}
	return nil
}
