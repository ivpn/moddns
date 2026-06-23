package model

const RECURSOR_SDNS = "sdns"
const RECURSOR_KNOT = "knot"
const RECURSOR_DEFAULT = RECURSOR_KNOT

var RECURSORS = []string{RECURSOR_SDNS, RECURSOR_KNOT}

type Advanced struct {
	Recursor string `json:"recursor" bson:"recursor" redis:"recursor" binding:"required"`
}
