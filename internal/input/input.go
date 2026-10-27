// Package input maps physical keys and gamepad buttons to the handheld's logical buttons,
// so UI code is identical on the device and in desktop-dev mode.
package input

// Button is a logical handheld button.
type Button int

const (
	Up Button = iota
	Down
	Left
	Right
	A
	B
	X
	Y
	L1
	R1
	L2
	R2
	Start
	Select
	Menu
)

var names = [...]string{"up", "down", "left", "right", "a", "b", "x", "y", "l1", "r1", "l2", "r2", "start", "select", "menu"}

func (b Button) String() string { return names[b] }

// keyNames maps SDL key names (sdl.GetKeyName) to buttons for desktop development.
var keyNames = map[string]Button{
	"Up": Up, "Down": Down, "Left": Left, "Right": Right,
	"Z": A, "X": B, "A": X, "S": Y,
	"Q": L1, "W": R1, "1": L2, "2": R2,
	"Return": Start, "Right Shift": Select, "Escape": Menu,
}

// FromKeyName resolves an SDL key name to a Button.
func FromKeyName(name string) (Button, bool) {
	b, ok := keyNames[name]
	return b, ok
}
