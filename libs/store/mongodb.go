package store

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"os"
	"reflect"
	"time"

	"github.com/google/uuid"
	"github.com/ivpn/dns/libs/store/migrator"
	"github.com/rs/zerolog/log"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/bsoncodec"
	"go.mongodb.org/mongo-driver/bson/bsonrw"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.mongodb.org/mongo-driver/mongo/readpref"
)

const (
	DbConnTimeout = 20 * time.Second
	DbPingTimeout = 20 * time.Second
)

// buildUUIDRegistry returns a BSON registry that encodes uuid.UUID as BSON
// binary subtype 0x04 (the canonical UUID subtype per the BSON spec).
//
// The default mongo-driver v1 codec treats uuid.UUID ([16]byte) as a generic
// byte array and writes it with subtype 0x00. That round-trips within a single
// Go service but breaks cross-driver queries and fails the BSON spec contract,
// so every mongo client constructed by this package installs the registry.
//
// The decoder is deliberately tolerant of legacy subtypes 0x00 and 0x03 so
// existing documents (written before the codec was installed) keep decoding
// while the data is being migrated to 0x04.
func buildUUIDRegistry() *bsoncodec.Registry {
	const uuidSubtype = byte(0x04)
	tUUID := reflect.TypeOf(uuid.UUID{})

	uuidEncoder := bsoncodec.ValueEncoderFunc(func(_ bsoncodec.EncodeContext, vw bsonrw.ValueWriter, val reflect.Value) error {
		if !val.IsValid() || val.Type() != tUUID {
			return bsoncodec.ValueEncoderError{Name: "uuidEncoder", Types: []reflect.Type{tUUID}, Received: val}
		}
		u := val.Interface().(uuid.UUID)
		return vw.WriteBinaryWithSubtype(u[:], uuidSubtype)
	})

	uuidDecoder := bsoncodec.ValueDecoderFunc(func(_ bsoncodec.DecodeContext, vr bsonrw.ValueReader, val reflect.Value) error {
		if !val.CanSet() || val.Type() != tUUID {
			return bsoncodec.ValueDecoderError{Name: "uuidDecoder", Types: []reflect.Type{tUUID}, Received: val}
		}
		data, subtype, err := vr.ReadBinary()
		if err != nil {
			return err
		}
		if subtype != 0x00 && subtype != 0x03 && subtype != 0x04 {
			return fmt.Errorf("unsupported binary subtype %#x for uuid.UUID", subtype)
		}
		u, err := uuid.FromBytes(data)
		if err != nil {
			return err
		}
		val.Set(reflect.ValueOf(u))
		return nil
	})

	reg := bson.NewRegistry()
	reg.RegisterTypeEncoder(tUUID, uuidEncoder)
	reg.RegisterTypeDecoder(tUUID, uuidDecoder)
	return reg
}

// MongoDB is a MongoDB database instance
type MongoDB struct {
	Config *Config
	Client *mongo.Client
}

// NewMongoDB creates a new MongoDB instance
func NewMongoDB(dbConfig *Config) (db *MongoDB, err error) {
	db = &MongoDB{
		Config: dbConfig,
	}
	err = db.connect()
	if err != nil {
		return nil, err
	}
	return db, nil
}

// Client returns the MongoDB client
func (db *MongoDB) GetClient() *mongo.Client {
	return db.Client
}

// Connect connects to the MongoDB database
func (db *MongoDB) connect() error {
	log.Info().Msg("Connecting to mongoDB")

	ctx, cancel := context.WithTimeout(context.Background(), DbConnTimeout)
	defer cancel()

	clientOpts := options.Client().ApplyURI(db.Config.DbURI)
	clientOpts.SetRegistry(buildUUIDRegistry())
	if db.Config.Username != "" && db.Config.Password != "" {
		log.Debug().Msg("Authenticating to mongoDB")
		credentials := buildMongoCredentials(db.Config)
		clientOpts.SetAuth(credentials)
	}
	if db.Config.TLSEnabled {
		log.Debug().Msg("TLS for mongoDB enabled")
		cert, err := tls.LoadX509KeyPair(db.Config.CertFile, db.Config.KeyFile)
		if err != nil {
			return fmt.Errorf("failed to load client certificate: %v", err)
		}

		caCert, err := os.ReadFile(db.Config.CACertFile)
		if err != nil {
			return fmt.Errorf("failed to load CA certificate: %v", err)
		}

		caCertPool := x509.NewCertPool()
		if ok := caCertPool.AppendCertsFromPEM(caCert); !ok {
			return fmt.Errorf("failed to append CA certificate")
		}

		tlsOpts := &tls.Config{
			Certificates:       []tls.Certificate{cert},
			RootCAs:            caCertPool,
			InsecureSkipVerify: db.Config.TLSInsecureSkipVerify,
		}

		clientOpts.SetTLSConfig(tlsOpts)
	}

	client, err := mongo.Connect(ctx, clientOpts)
	if err != nil {
		return err
	}
	ctx, cancel = context.WithTimeout(context.Background(), DbPingTimeout)
	defer cancel()
	if err = client.Ping(ctx, readpref.Primary()); err != nil {
		return err
	}

	db.Client = client
	return nil
}

// buildMongoCredentials builds mongo credential object using config (extracted for testing)
func buildMongoCredentials(cfg *Config) options.Credential {
	authSource := cfg.AuthSource
	if authSource == "" {
		authSource = "dns"
	}
	return options.Credential{
		Username:   cfg.Username,
		Password:   cfg.Password,
		AuthSource: authSource,
	}
}

// Disconnect disconnects from the MongoDB database
func (db *MongoDB) Disconnect() error {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if err := db.Client.Disconnect(ctx); err != nil {
		return err
	}
	return nil
}

// Migrate runs migrations
func (db *MongoDB) Migrate() error {
	log.Info().Msg("Running DB migrations")
	migrator, err := migrator.NewMigrator(db.Client, db.Config.Name, db.Config.MigrationsSource)
	if err != nil {
		return err
	}
	return migrator.Migrate()
}
