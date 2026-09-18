package worker

import (
	"bytes"
	"encoding/json"
	"errors"
	"sort"
)

// boundedJSONWriter stops before retaining more than the configured output limit.
type boundedJSONWriter struct {
	buffer bytes.Buffer
	limit  int
}

func encodeBounded(value any, limit int) (json.RawMessage, error) {
	writer := boundedJSONWriter{limit: limit}
	if err := writer.value(value); err != nil {
		return nil, err
	}
	return writer.buffer.Bytes(), nil
}
func (w *boundedJSONWriter) write(value []byte) error {
	if len(value) > w.limit-w.buffer.Len() {
		return errors.New("RESULT_TOO_LARGE")
	}
	_, err := w.buffer.Write(value)
	return err
}
func (w *boundedJSONWriter) value(value any) error {
	switch item := value.(type) {
	case json.RawMessage:
		return w.write(item)
	case map[string]any:
		if err := w.write([]byte("{")); err != nil {
			return err
		}
		keys := make([]string, 0, len(item))
		for key := range item {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		for index, key := range keys {
			if index > 0 {
				if err := w.write([]byte(",")); err != nil {
					return err
				}
			}
			if err := w.value(key); err != nil {
				return err
			}
			if err := w.write([]byte(":")); err != nil {
				return err
			}
			if err := w.value(item[key]); err != nil {
				return err
			}
		}
		return w.write([]byte("}"))
	case []any:
		if err := w.write([]byte("[")); err != nil {
			return err
		}
		for index, element := range item {
			if index > 0 {
				if err := w.write([]byte(",")); err != nil {
					return err
				}
			}
			if err := w.value(element); err != nil {
				return err
			}
		}
		return w.write([]byte("]"))
	default:
		encoded, err := json.Marshal(value)
		if err != nil {
			return errors.New("RESULT_SERIALIZATION_FAILED")
		}
		return w.write(encoded)
	}
}
