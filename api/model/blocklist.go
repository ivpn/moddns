package model

import (
	"fmt"
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

const (
	BlocklistTypePublic  = "public"
	BlocklistTypePrivate = "private"
)

// Blocklist represents the metadata of a blocklist without the actual content
type Blocklist struct {
	ID           primitive.ObjectID `json:"id" bson:"_id"`
	BlocklistID  string             `json:"blocklist_id" bson:"blocklist_id" binding:"required"`
	Name         string             `json:"name" binding:"required"`        // conventional blocklist name, displayed to the user
	Description  string             `json:"description" binding:"required"` // displayed to the user
	Entries      int                `json:"entries" bson:"entries"`
	Homepage     string             `json:"homepage" bson:"homepage"`
	SourceUrl    string             `json:"source_url" bson:"source_url"`
	LastModified time.Time          `json:"last_modified" bson:"last_modified"`
	Tags         []string           `json:"tags" bson:"tags"`
	Type         string             `json:"type" bson:"type"`           // ownership: public (platform-provided) or private (user-uploaded)
	Kind         string             `json:"kind" bson:"kind"`           // general, category, security
	Category     string             `json:"category" bson:"category"`   // category key (only when kind=category)
	Intensity    string             `json:"intensity" bson:"intensity"` // basic, comprehensive, restrictive
	Default      bool               `json:"default" bson:"default"`     // default blocklist is enabled when profile is created
}

// NewBlocklist creates a new blocklist
func NewBlocklist(blocklistId, name, description, website, sourceUrl, blocklistType string, tags []string, isDefault bool) (*Blocklist, error) {
	if blocklistId == "" {
		return nil, fmt.Errorf("blocklist_id cannot be empty")
	}

	if blocklistType == "" {
		blocklistType = BlocklistTypePublic
	}

	return &Blocklist{
		ID:           primitive.NewObjectID(),
		BlocklistID:  blocklistId,
		Name:         name,
		Description:  description,
		Homepage:     website,
		SourceUrl:    sourceUrl,
		Tags:         tags,
		Default:      isDefault,
		Type:         blocklistType,
		LastModified: time.Now(),
	}, nil
}
