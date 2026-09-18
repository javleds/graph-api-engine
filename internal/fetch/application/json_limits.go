package application

import (
	"bytes"
	"encoding/json"
	"github.com/javleds/api-engine/internal/fetch/domain"
	"io"
)

func validateJSONLimits(body []byte, maxDepth, maxValues int) *domain.UpstreamFailure {
	decoder := json.NewDecoder(bytes.NewReader(body))
	decoder.UseNumber()
	depth, values := 0, 0
	for {
		token, err := decoder.Token()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return &domain.UpstreamFailure{Code: "UPSTREAM_INVALID_JSON"}
		}
		if delimiter, ok := token.(json.Delim); ok {
			if delimiter == '{' || delimiter == '[' {
				depth++
				values++
			} else {
				depth--
			}
		} else {
			values++
		}
		if depth > maxDepth || values > maxValues {
			return &domain.UpstreamFailure{Code: "UPSTREAM_INVALID_RESPONSE"}
		}
	}
}
