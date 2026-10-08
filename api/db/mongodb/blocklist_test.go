package mongodb

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/bson"
)

func TestBuildBlocklistSortSpec(t *testing.T) {
	testCases := []struct {
		name     string
		sortBy   string
		expected bson.D
	}{
		{name: "default-updated", sortBy: "updated", expected: bson.D{{Key: "last_modified", Value: -1}}},
		{name: "name", sortBy: "name", expected: bson.D{{Key: "name", Value: 1}, {Key: "last_modified", Value: -1}}},
		{name: "entries", sortBy: "entries", expected: bson.D{{Key: "entries", Value: -1}, {Key: "last_modified", Value: -1}}},
		{name: "unknown", sortBy: "random", expected: bson.D{{Key: "last_modified", Value: -1}}},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.expected, buildBlocklistSortSpec(tc.sortBy))
		})
	}
}

// tableRef: api-endpoint-behaviour #J24
func TestBuildBlocklistFilter(t *testing.T) {
	f, err := buildBlocklistFilter(map[string]any{"blocklist_ids": []string{"a", "b"}})
	require.NoError(t, err)
	assert.Equal(t, bson.D{{Key: "blocklist_id", Value: bson.D{{Key: "$in", Value: []string{"a", "b"}}}}}, f)

	f, err = buildBlocklistFilter(map[string]any{"blocklist_id": "a"})
	require.NoError(t, err)
	assert.Equal(t, bson.D{{Key: "blocklist_id", Value: "a"}}, f)

	f, err = buildBlocklistFilter(map[string]any{"default": "true"})
	require.NoError(t, err)
	assert.Equal(t, bson.D{{Key: "default", Value: true}}, f)

	f, err = buildBlocklistFilter(map[string]any{})
	require.NoError(t, err)
	assert.Equal(t, bson.D{}, f)
}

// tableRef: api-endpoint-behaviour #J24
func TestBlocklistRepository_GetByIDs(t *testing.T) {
	client, container := startTestMongo(t)
	ctx := context.Background()
	defer func() { _ = container.Terminate(ctx) }()
	repo := NewBlocklistRepository(client, "dns_test_blocklists", "blocklists_metadata")
	coll := client.Database("dns_test_blocklists").Collection("blocklists_metadata")
	_, err := coll.InsertMany(ctx, []any{
		bson.D{{Key: "blocklist_id", Value: "a"}, {Key: "name", Value: "List A"}, {Key: "last_modified", Value: time.Now()}},
		bson.D{{Key: "blocklist_id", Value: "b"}, {Key: "name", Value: "List B"}, {Key: "last_modified", Value: time.Now()}},
		bson.D{{Key: "blocklist_id", Value: "c"}, {Key: "name", Value: "List C"}, {Key: "last_modified", Value: time.Now()}},
	})
	require.NoError(t, err)

	got, err := repo.Get(ctx, map[string]any{"blocklist_ids": []string{"a", "c", "missing"}}, "name")
	require.NoError(t, err)
	names := map[string]string{}
	for _, b := range got {
		names[b.BlocklistID] = b.Name
	}
	assert.Equal(t, map[string]string{"a": "List A", "c": "List C"}, names)
}
