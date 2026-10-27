package input

import "testing"

func TestFromKeyName(t *testing.T) {
	cases := map[string]Button{
		"Up": Up, "Down": Down, "Left": Left, "Right": Right,
		"Z": A, "X": B, "A": X, "S": Y,
		"Q": L1, "W": R1,
		"Return": Start, "Right Shift": Select, "Escape": Menu,
	}
	for name, want := range cases {
		got, ok := FromKeyName(name)
		if !ok || got != want {
			t.Errorf("FromKeyName(%q) = %v,%v want %v,true", name, got, ok, want)
		}
	}
	if _, ok := FromKeyName("F13"); ok {
		t.Error("unmapped key should not resolve")
	}
}

func TestButtonString(t *testing.T) {
	if Start.String() != "start" || L1.String() != "l1" {
		t.Fatalf("unexpected names: %s %s", Start, L1)
	}
}
