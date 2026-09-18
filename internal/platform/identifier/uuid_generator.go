package identifier

import "github.com/google/uuid"

type UUIDGenerator struct{}

func (UUIDGenerator) NewID() (string, error) { id, err := uuid.NewV7(); return id.String(), err }
