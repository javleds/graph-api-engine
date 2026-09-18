package dto

type DefinitionDTO struct {
	Version int              `json:"version"`
	API     APIDefinitionDTO `json:"api"`
	Calls   RootCallsDTO     `json:"calls"`
}
