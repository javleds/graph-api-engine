package domain

import "encoding/json"

type V2ChunkPage struct {
	Chunks       []json.RawMessage `json:"chunks"`
	NextCursor   string            `json:"nextCursor"`
	HasMore      bool              `json:"hasMore"`
	Terminal     bool              `json:"terminal"`
	LastSequence string            `json:"lastSequence"`
}
