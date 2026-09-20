package application

import "bytes"

type boundedPayload struct {
	buffer  bytes.Buffer
	maximum int
}

func (w *boundedPayload) Write(p []byte) (int, error) {
	if len(p) > w.maximum-w.buffer.Len() {
		return 0, errNodePayloadTooLarge
	}
	return w.buffer.Write(p)
}
