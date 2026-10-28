package downloadui

import "testing"

func TestShouldReturn(t *testing.T) {
	cases := []struct {
		name                                   string
		finished, userLeft, cancelled, stopped bool
		want                                   bool
	}{
		{"running", false, false, false, false, false},
		{"finished on its own: stay and show the summary", true, false, false, true, false},
		{"finished, user presses back", true, true, false, true, true},
		{"cancelled but the queue is still winding down", false, false, true, false, false},
		{"cancelled and stopped", false, false, true, true, true},
		{"user left but the queue has not stopped yet", true, true, false, false, false},
	}
	for _, c := range cases {
		if got := shouldReturn(c.finished, c.userLeft, c.cancelled, c.stopped); got != c.want {
			t.Errorf("%s: shouldReturn = %v, want %v", c.name, got, c.want)
		}
	}
}
