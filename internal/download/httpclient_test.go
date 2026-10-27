package download

import "testing"

func TestHTTPClientHasNoOverallTimeout(t *testing.T) {
	if got := HTTPClient().Timeout; got != 0 {
		t.Fatalf("HTTPClient().Timeout = %v, want 0 (a ROM body can take minutes)", got)
	}
}
