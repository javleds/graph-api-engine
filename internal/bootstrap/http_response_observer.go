package bootstrap

import "net/http"

type httpResponseObserver struct {
	http.ResponseWriter
	status int
}

func (w *httpResponseObserver) WriteHeader(status int) {
	w.status = status
	w.ResponseWriter.WriteHeader(status)
}
func (w *httpResponseObserver) Write(data []byte) (int, error) {
	if w.status == 0 {
		w.WriteHeader(http.StatusOK)
	}
	return w.ResponseWriter.Write(data)
}
func (w *httpResponseObserver) Unwrap() http.ResponseWriter { return w.ResponseWriter }
