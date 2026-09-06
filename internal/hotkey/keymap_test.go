package hotkey

import (
	"testing"

	"github.com/holoplot/go-evdev"
)

func TestParseCombo(t *testing.T) {
	combo, err := ParseCombo("ctrl+space")
	if err != nil {
		t.Fatalf("ParseCombo('ctrl+space') failed: %v", err)
	}

	if len(combo) != 2 {
		t.Fatalf("expected 2 slots, got %d", len(combo))
	}

	pressed := make(map[evdev.EvCode]bool)

	// Neither pressed
	if combo.IsActive(pressed) {
		t.Errorf("expected combo not to be active")
	}

	// Only ctrl pressed
	pressed[evdev.KEY_LEFTCTRL] = true
	if combo.IsActive(pressed) {
		t.Errorf("expected combo not to be active with only ctrl")
	}

	// Both pressed
	pressed[evdev.KEY_SPACE] = true
	if !combo.IsActive(pressed) {
		t.Errorf("expected combo to be active with leftctrl and space")
	}

	// Release space
	pressed[evdev.KEY_SPACE] = false
	if combo.IsActive(pressed) {
		t.Errorf("expected combo to become inactive after releasing space")
	}

	// Test Right Ctrl + Space
	delete(pressed, evdev.KEY_LEFTCTRL)
	pressed[evdev.KEY_RIGHTCTRL] = true
	pressed[evdev.KEY_SPACE] = true
	if !combo.IsActive(pressed) {
		t.Errorf("expected combo to be active with rightctrl and space")
	}
}

func TestParseVariousCombos(t *testing.T) {
	cases := []string{
		"alt+v",
		"super+space",
		"ctrl+shift+f9",
		"KEY_LEFTCTRL+KEY_SPACE",
		"capslock",
		"mod+r",
	}

	for _, c := range cases {
		combo, err := ParseCombo(c)
		if err != nil {
			t.Errorf("ParseCombo(%q) failed: %v", c, err)
		}
		if len(combo) == 0 {
			t.Errorf("ParseCombo(%q) returned empty combo", c)
		}
	}
}
