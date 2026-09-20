package dto

import "github.com/javleds/api-engine/internal/execution/domain"

type V2ResultResponse struct {
	domain.V2Result
	Links V2ResultLinks `json:"links"`
}
