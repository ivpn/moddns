package model

import (
	"fmt"
	"strings"
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

const (
	BlocklistTypePublic  = "public"
	BlocklistTypePrivate = "private"
)

// BlocklistMetadata is a blocklist model
type BlocklistMetadata struct {
	ID           primitive.ObjectID `json:"id" bson:"_id"`
	BlocklistID  string             `json:"blocklist_id" bson:"blocklist_id" binding:"required"`
	Name         string             `json:"name" binding:"required"`        // conventional blocklist name, displayed to the user
	Description  string             `json:"description" binding:"required"` // displayed to the user
	Entries      int                `json:"entries"`
	Homepage     string             `json:"homepage"`
	SourceUrl    string             `json:"source_url" bson:"source_url"`
	LastModified time.Time          `json:"last_modified" bson:"last_modified"`
	Version      string             `json:"version"`
	Tags         []string           `json:"tags"`
	Type         string             `json:"type"` // ownership: public (platform-provided) or private (user-uploaded)
	Kind         string             `json:"kind" bson:"kind"`           // general, category, security
	Category     string             `json:"category" bson:"category"`   // category key (only when kind=category)
	Intensity    string             `json:"intensity" bson:"intensity"` // basic, comprehensive, restrictive
	Default      bool               `json:"default"`                    // default blocklist is enabled when profile is created
	Syntax       string             `json:"syntax"`
	Schedule     string             `json:"schedule"`
}

// BlocklistContent contains the actual blocklist data
type BlocklistContent struct {
	ID          primitive.ObjectID `json:"id" bson:"_id"`
	BlocklistID string             `json:"blocklist_id" bson:"blocklist_id"`
	Part        int                `json:"part" bson:"part"`
	Data        []byte             `json:"-" bson:"data"`
}

// NewBlocklistContent creates a new blocklist content
func NewBlocklistContent(blocklistId string, partNum int, data []string) (*BlocklistContent, error) {
	if blocklistId == "" {
		return nil, fmt.Errorf("blocklist_id cannot be empty")
	}

	return &BlocklistContent{
		ID:          primitive.NewObjectID(),
		BlocklistID: blocklistId,
		Part:        partNum,
		Data:        []byte(strings.Join(data, "\n")),
	}, nil
}
