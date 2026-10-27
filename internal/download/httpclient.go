package download

import "net/http"

// HTTPClient returns the HTTP client for ROM downloads. It has no overall
// timeout: http.Client.Timeout covers the whole response body, and a ROM can
// legitimately take minutes. Cancellation is the context given to Queue.Run,
// which stops the body at the next chunk.
func HTTPClient() *http.Client {
	return &http.Client{}
}
