package migrator

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/golang-migrate/migrate/v4"
	"github.com/golang-migrate/migrate/v4/database/mongodb"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"github.com/rs/zerolog/log"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

const (
	// lockCollectionName mirrors mongodb.DefaultLockingCollection. Duplicated
	// here so we can reach the collection without importing the driver's
	// private internals.
	lockCollectionName = "migrate_advisory_lock"
	// lockTTLSeconds bounds how long a stale lock survives a crashed migration.
	// Must exceed the Locking.Timeout below (300s) plus the longest realistic
	// migration runtime so a slow-but-healthy migration is never evicted while
	// still running.
	lockTTLSeconds = 600
)

type DBMigrator struct {
	migrator *migrate.Migrate
}

// migrateLogger adapts zerolog into golang-migrate's Logger interface
// (Printf + Verbose). Without this, the library swallows its own per-step
// logs ("scheduling", "running", "finished") and we only see what our wrapper
// chooses to print — which makes "stuck dirty" failures invisible.
type migrateLogger struct{}

func (migrateLogger) Printf(format string, v ...interface{}) {
	log.Info().Msgf("migrate: "+format, v...)
}

func (migrateLogger) Verbose() bool { return true }

func NewMigrator(dbClient *mongo.Client, dbName, migrationsSource string) (*DBMigrator, error) {
	driverMongo, err := mongodb.WithInstance(dbClient, &mongodb.Config{
		DatabaseName: dbName,
		// Advisory locking serializes Migrate() across concurrently-starting
		// service instances. Without it, two pods can read the same clean
		// version, both mark dirty, and one observes the other's dirty state
		// and crashes — leaving the migrations table stuck dirty even though
		// the underlying operation succeeded. Timeout is generous so a slow
		// index build on a large collection doesn't starve a peer's startup.
		Locking: mongodb.Locking{
			Enabled: true,
			Timeout: 300,
		},
	})
	if err != nil {
		return nil, err
	}
	// WithInstance has just created the lock collection. Add a TTL on its
	// created_at field so a stale lock (from a crashed migration) self-cleans
	// instead of blocking every subsequent deploy. Best-effort: a TTL failure
	// here would only impair self-healing, not correctness, so it must not
	// prevent service startup.
	ensureLockTTL(dbClient, dbName)

	m, err := migrate.NewWithDatabaseInstance(
		migrationsSource,
		dbName, driverMongo)
	if err != nil {
		return nil, err
	}
	m.Log = migrateLogger{}
	return &DBMigrator{migrator: m}, nil
}

// ensureLockTTL creates a TTL index on migrate_advisory_lock.created_at if
// one doesn't already exist. createIndex is idempotent for matching specs,
// so this is safe to call on every startup. Any error is logged and swallowed
// — the worst-case effect of failure is that stale locks need manual cleanup,
// which is the pre-patch status quo.
func ensureLockTTL(dbClient *mongo.Client, dbName string) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	coll := dbClient.Database(dbName).Collection(lockCollectionName)
	ttl := int32(lockTTLSeconds)
	_, err := coll.Indexes().CreateOne(ctx, mongo.IndexModel{
		Keys: bson.D{{Key: "created_at", Value: 1}},
		Options: options.Index().
			SetName("created_at_ttl").
			SetExpireAfterSeconds(ttl),
	})
	if err != nil {
		log.Warn().Err(err).Str("collection", lockCollectionName).Msg("Failed to ensure TTL index on migration lock collection; stale locks may need manual cleanup")
		return
	}
	log.Debug().Str("collection", lockCollectionName).Int32("ttl_seconds", ttl).Msg("Ensured TTL index on migration lock collection")
}

func (m *DBMigrator) Migrate() error {
	versionBefore, dirtyBefore, err := m.migrator.Version()
	if err != nil && err != migrate.ErrNilVersion {
		log.Error().Err(err).Msg("Failed to get DB version before migrations")
		return err
	}
	log.Info().Uint("version", versionBefore).Bool("dirty", dirtyBefore).Msg("DB version before migrations")

	upErr := m.migrator.Up()
	switch {
	case upErr == nil:
		log.Info().Msg("Migrations applied successfully")
	case errors.Is(upErr, migrate.ErrNoChange):
		log.Info().Msg("No migrations to apply")
	default:
		// Type-discriminate so the operator gets an actionable message
		// instead of a generic "Failed to apply DB migrations".
		var dirtyErr migrate.ErrDirty
		switch {
		case errors.As(upErr, &dirtyErr):
			log.Error().
				Err(upErr).
				Int("dirty_version", dirtyErr.Version).
				Msg("Cannot migrate: database is in dirty state. Recover with: db.schema_migrations.updateOne({}, {$set: {dirty: false}})")
		case errors.Is(upErr, migrate.ErrLocked):
			log.Error().Err(upErr).Msg("Cannot acquire migration lock (another instance is migrating or stale lock — check migrate_advisory_lock collection)")
		default:
			log.Error().Err(upErr).Str("error_type", fmt.Sprintf("%T", upErr)).Msg("Failed to apply DB migrations")
		}
		return upErr
	}

	versionAfter, dirtyAfter, verErr := m.migrator.Version()
	if verErr != nil {
		log.Error().Err(verErr).Msg("Failed to get DB version after migrations")
		return verErr
	}
	log.Info().Uint("version", versionAfter).Bool("dirty", dirtyAfter).Msg("DB version after migrations")

	// Previously we returned nil even when the post-migration state was dirty,
	// silently leaving the API running against a half-migrated schema. Surface
	// it as a hard failure so main.go's log.Panic fires with a clear message.
	if dirtyAfter {
		return fmt.Errorf("migration completed but schema_migrations still marked dirty at version %d — manual recovery required", versionAfter)
	}

	return nil
}
