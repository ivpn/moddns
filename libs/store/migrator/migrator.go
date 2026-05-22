package migrator

import (
	"github.com/golang-migrate/migrate/v4"
	"github.com/golang-migrate/migrate/v4/database/mongodb"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"github.com/rs/zerolog/log"
	"go.mongodb.org/mongo-driver/mongo"
)

type DBMigrator struct {
	migrator *migrate.Migrate
}

func NewMigrator(dbClient *mongo.Client, dbName, migrationsSource string) (*DBMigrator, error) {
	driverMongo, err := mongodb.WithInstance(dbClient, &mongodb.Config{
		DatabaseName: dbName,
		Locking: mongodb.Locking{
			Enabled: true,
			Timeout: 300,
		},
	})
	if err != nil {
		return nil, err
	}
	m, err := migrate.NewWithDatabaseInstance(
		migrationsSource,
		dbName, driverMongo)
	if err != nil {
		return nil, err
	}
	return &DBMigrator{migrator: m}, nil
}

func (m *DBMigrator) Migrate() error {
	versionBefore, dirty, err := m.migrator.Version()
	if err != nil {
		if err != migrate.ErrNilVersion {
			log.Error().Err(err).Msg("Failed to get DB version before migrations")
			return err
		}
	}
	log.Info().Uint("version", versionBefore).Bool("dirty", dirty).Msgf("DB version before migrations: %v", versionBefore)

	if err := m.migrator.Up(); err != nil {
		if err == migrate.ErrNoChange {
			log.Info().Msg("No migrations to apply")
		} else {
			log.Error().Err(err).Msg("Failed to apply DB migrations")
			return err
		}
	}

	versionAfter, dirty, err := m.migrator.Version()
	if err != nil {
		log.Error().Err(err).Msg("Failed to get DB version after migrations")
		return err
	}
	log.Info().Uint("version", versionAfter).Bool("dirty", dirty).Msgf("DB version after migrations: %v", versionAfter)
	return nil
}
