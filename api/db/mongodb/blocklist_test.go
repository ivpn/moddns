package mongodb

import (
	"testing"

	"github.com/stretchr/testify/assert"
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
