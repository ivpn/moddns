package requests

// BlocklistsQueryParams holds query parameters for GET /blocklists endpoint.
type BlocklistsQueryParams struct {
	SortBy string `json:"sort_by" validate:"omitempty,oneof=updated name entries"`
}
