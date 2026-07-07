package model

const RECURSOR_SDNS = "sdns"
const RECURSOR_KNOT = "knot"
const RECURSOR_DEFAULT = RECURSOR_KNOT

// RECURSORS is the source of truth for allowed recursors. Keep it in sync with
// the `enums` tag on Advanced.Recursor below, which swaggo publishes into the
// OpenAPI spec so the generated clients (and the web app) derive the available
// recursors instead of hardcoding them.
var RECURSORS = []string{RECURSOR_SDNS, RECURSOR_KNOT}

type Advanced struct {
	Recursor string `json:"recursor" bson:"recursor" redis:"recursor" binding:"required" enums:"sdns,knot"`
}
