package model

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/bson"
)

// TestPrivacy_MarshalJSON_NilSlicesRenderAsEmptyArrays verifies the API never
// returns `null` for blocklists/services: nil slices serialize as [].
func TestPrivacy_MarshalJSON_NilSlicesRenderAsEmptyArrays(t *testing.T) {
	p := Privacy{
		Blocklists:  nil,
		Services:    nil,
		DefaultRule: DEFAULT_RULE_ALLOW,
	}
	raw, err := json.Marshal(p)
	require.NoError(t, err)

	var out map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(raw, &out))
	assert.JSONEq(t, "[]", string(out["blocklists"]), "nil blocklists must render as []")
	assert.JSONEq(t, "[]", string(out["services"]), "nil services must render as []")

	// Populated slices must round-trip unchanged.
	p2 := Privacy{Blocklists: []string{"bl1"}, Services: []string{"spotify"}, DefaultRule: DEFAULT_RULE_ALLOW}
	raw2, err := json.Marshal(p2)
	require.NoError(t, err)
	var back Privacy
	require.NoError(t, json.Unmarshal(raw2, &back))
	assert.Equal(t, []string{"bl1"}, back.Blocklists)
	assert.Equal(t, []string{"spotify"}, back.Services)
}

// TestPrivacy_MarshalJSON_ViaPointer verifies the marshaler is used when Privacy
// is reached through a pointer field (as it is in ProfileSettings).
func TestPrivacy_MarshalJSON_ViaPointer(t *testing.T) {
	wrapper := struct {
		Privacy *Privacy `json:"privacy"`
	}{Privacy: &Privacy{DefaultRule: DEFAULT_RULE_ALLOW}}
	raw, err := json.Marshal(wrapper)
	require.NoError(t, err)
	assert.Contains(t, string(raw), `"services":[]`)
	assert.Contains(t, string(raw), `"blocklists":[]`)
	assert.NotContains(t, string(raw), `null`)
}

func TestPrivacy_Services_Array(t *testing.T) {
	// Current format: services is ["svc1", "svc2"]
	doc := bson.M{"services": bson.A{"svc1", "svc2"}}
	raw, err := bson.Marshal(doc)
	require.NoError(t, err)

	var result struct {
		Services []string `bson:"services"`
	}
	err = bson.Unmarshal(raw, &result)
	require.NoError(t, err)
	assert.Equal(t, []string{"svc1", "svc2"}, result.Services)
}

func TestPrivacy_Services_EmptyArray(t *testing.T) {
	doc := bson.M{"services": bson.A{}}
	raw, err := bson.Marshal(doc)
	require.NoError(t, err)

	var result struct {
		Services []string `bson:"services"`
	}
	err = bson.Unmarshal(raw, &result)
	require.NoError(t, err)
	assert.Equal(t, []string{}, result.Services)
}

func TestPrivacy_Services_Nil(t *testing.T) {
	doc := bson.M{}
	raw, err := bson.Marshal(doc)
	require.NoError(t, err)

	var result struct {
		Services []string `bson:"services"`
	}
	err = bson.Unmarshal(raw, &result)
	require.NoError(t, err)
	assert.Nil(t, result.Services)
}
