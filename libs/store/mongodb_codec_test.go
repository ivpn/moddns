package store

import (
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// docWithUUIDID mirrors how api/model/subscription.go maps uuid.UUID into _id.
type docWithUUIDID struct {
	ID uuid.UUID `bson:"_id"`
}

func TestUUIDCodec_EncodesSubtype04(t *testing.T) {
	reg := buildUUIDRegistry()
	id := uuid.New()

	data, err := bson.MarshalWithRegistry(reg, docWithUUIDID{ID: id})
	require.NoError(t, err)

	// Decode as raw bson.D to inspect the stored subtype independently of the
	// registry's decoder.
	var raw bson.D
	require.NoError(t, bson.Unmarshal(data, &raw))
	require.Len(t, raw, 1)
	require.Equal(t, "_id", raw[0].Key)

	bin, ok := raw[0].Value.(primitive.Binary)
	require.True(t, ok, "expected _id to decode as primitive.Binary, got %T", raw[0].Value)
	require.Equal(t, byte(0x04), bin.Subtype, "expected UUID subtype 0x04")
	require.Equal(t, id[:], bin.Data)

	// Round-trip into a typed struct and ensure the UUID comes back intact.
	var decoded docWithUUIDID
	require.NoError(t, bson.UnmarshalWithRegistry(reg, data, &decoded))
	require.Equal(t, id, decoded.ID)
}

func TestUUIDCodec_DecodesLegacySubtype00(t *testing.T) {
	reg := buildUUIDRegistry()
	id := uuid.New()

	// Hand-craft a BSON document with the legacy subtype 0x00 that the default
	// mongo-driver codec used to produce for uuid.UUID values.
	legacy := bson.D{{Key: "_id", Value: primitive.Binary{Subtype: 0x00, Data: id[:]}}}
	data, err := bson.Marshal(legacy)
	require.NoError(t, err)

	var decoded docWithUUIDID
	require.NoError(t, bson.UnmarshalWithRegistry(reg, data, &decoded))
	require.Equal(t, id, decoded.ID)
}

func TestUUIDCodec_DecodesLegacySubtype03(t *testing.T) {
	reg := buildUUIDRegistry()
	id := uuid.New()

	// Subtype 0x03 is the deprecated "old UUID" subtype. Some drivers still
	// emit it; the tolerant decoder should accept it.
	legacy := bson.D{{Key: "_id", Value: primitive.Binary{Subtype: 0x03, Data: id[:]}}}
	data, err := bson.Marshal(legacy)
	require.NoError(t, err)

	var decoded docWithUUIDID
	require.NoError(t, bson.UnmarshalWithRegistry(reg, data, &decoded))
	require.Equal(t, id, decoded.ID)
}

func TestUUIDCodec_RejectsInvalidSubtype(t *testing.T) {
	reg := buildUUIDRegistry()
	id := uuid.New()

	// Any non-UUID binary subtype should be rejected loudly rather than
	// silently copying bytes into a uuid.UUID.
	bad := bson.D{{Key: "_id", Value: primitive.Binary{Subtype: 0x05, Data: id[:]}}}
	data, err := bson.Marshal(bad)
	require.NoError(t, err)

	var decoded docWithUUIDID
	err = bson.UnmarshalWithRegistry(reg, data, &decoded)
	require.Error(t, err)
	require.Contains(t, err.Error(), "unsupported binary subtype")
}
