package domain

import (
	"time"
)

// Provenance tracks the source and version of data
type Provenance struct {
	Source        string    `json:"source"`
	SourceVersion string    `json:"source_version"`
	RetrievedAt   time.Time `json:"retrieved_at"`
}