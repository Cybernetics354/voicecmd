package hotkey

import (
	"fmt"
	"slices"
	"strings"

	"github.com/holoplot/go-evdev"
)

// KeySlot represents a group of acceptable key codes for a single combination slot
// (e.g., Left Control OR Right Control for "ctrl").
type KeySlot []evdev.EvCode

// KeyCombo is a list of required key slots that must all be pressed simultaneously.
type KeyCombo []KeySlot

// Matches returns true if this slot is satisfied given the currently pressed keys.
func (ks KeySlot) Matches(pressed map[evdev.EvCode]bool) bool {
	for _, code := range ks {
		if pressed[code] {
			return true
		}
	}
	return false
}

// IsActive returns true if all slots in the combination are satisfied.
func (kc KeyCombo) IsActive(pressed map[evdev.EvCode]bool) bool {
	if len(kc) == 0 {
		return false
	}
	for _, slot := range kc {
		if !slot.Matches(pressed) {
			return false
		}
	}
	return true
}

// ContainsCode returns true if code belongs to any slot in this combination.
func (kc KeyCombo) ContainsCode(code evdev.EvCode) bool {
	for _, slot := range kc {
		if slices.Contains(slot, code) {
			return true
		}
	}
	return false
}

// ParseCombo parses a human-readable combination string (e.g. "ctrl+space", "alt+v", "KEY_LEFTCTRL+KEY_SPACE")
// into a KeyCombo.
func ParseCombo(comboStr string) (KeyCombo, error) {
	trimmed := strings.TrimSpace(comboStr)
	if trimmed == "" {
		return nil, fmt.Errorf("empty key combination")
	}

	parts := strings.Split(trimmed, "+")
	var combo KeyCombo

	for _, p := range parts {
		token := strings.TrimSpace(strings.ToLower(p))
		if token == "" {
			continue
		}

		slot, err := parseKeyToken(token)
		if err != nil {
			return nil, fmt.Errorf("invalid key %q in combo %q: %w", p, comboStr, err)
		}
		combo = append(combo, slot)
	}

	if len(combo) == 0 {
		return nil, fmt.Errorf("no valid keys found in %q", comboStr)
	}

	return combo, nil
}

func parseKeyToken(token string) (KeySlot, error) {
	upper := strings.ToUpper(token)

	// Friendly alias mappings
	switch token {
	case "ctrl", "control":
		return KeySlot{evdev.KEY_LEFTCTRL, evdev.KEY_RIGHTCTRL}, nil
	case "leftctrl", "left_ctrl", "lctrl":
		return KeySlot{evdev.KEY_LEFTCTRL}, nil
	case "rightctrl", "right_ctrl", "rctrl":
		return KeySlot{evdev.KEY_RIGHTCTRL}, nil

	case "alt":
		return KeySlot{evdev.KEY_LEFTALT, evdev.KEY_RIGHTALT}, nil
	case "leftalt", "left_alt", "lalt":
		return KeySlot{evdev.KEY_LEFTALT}, nil
	case "rightalt", "right_alt", "ralt", "altgr":
		return KeySlot{evdev.KEY_RIGHTALT}, nil

	case "shift":
		return KeySlot{evdev.KEY_LEFTSHIFT, evdev.KEY_RIGHTSHIFT}, nil
	case "leftshift", "left_shift", "lshift":
		return KeySlot{evdev.KEY_LEFTSHIFT}, nil
	case "rightshift", "right_shift", "rshift":
		return KeySlot{evdev.KEY_RIGHTSHIFT}, nil

	case "super", "meta", "win", "windows", "mod":
		return KeySlot{evdev.KEY_LEFTMETA, evdev.KEY_RIGHTMETA}, nil
	case "leftmeta", "leftsuper", "lmeta":
		return KeySlot{evdev.KEY_LEFTMETA}, nil
	case "rightmeta", "rightsuper", "rmeta":
		return KeySlot{evdev.KEY_RIGHTMETA}, nil

	case "space", "spacebar":
		return KeySlot{evdev.KEY_SPACE}, nil
	case "enter", "return":
		return KeySlot{evdev.KEY_ENTER}, nil
	case "esc", "escape":
		return KeySlot{evdev.KEY_ESC}, nil
	case "tab":
		return KeySlot{evdev.KEY_TAB}, nil
	case "backspace":
		return KeySlot{evdev.KEY_BACKSPACE}, nil
	case "capslock", "caps_lock", "caps":
		return KeySlot{evdev.KEY_CAPSLOCK}, nil
	}

	// Single letter a-z
	if len(token) == 1 && token[0] >= 'a' && token[0] <= 'z' {
		keyName := "KEY_" + upper
		if code, ok := evdev.KEYFromString[keyName]; ok {
			return KeySlot{code}, nil
		}
	}

	// Single digit 0-9
	if len(token) == 1 && token[0] >= '0' && token[0] <= '9' {
		keyName := "KEY_" + token
		if code, ok := evdev.KEYFromString[keyName]; ok {
			return KeySlot{code}, nil
		}
	}

	// Function keys f1 - f24
	if len(token) >= 2 && token[0] == 'f' && token[1] >= '1' && token[1] <= '9' {
		keyName := "KEY_" + upper
		if code, ok := evdev.KEYFromString[keyName]; ok {
			return KeySlot{code}, nil
		}
	}

	// Direct evdev name: e.g. "KEY_LEFTCTRL", "KEY_F9", etc.
	if code, ok := evdev.KEYFromString[upper]; ok {
		return KeySlot{code}, nil
	}
	if code, ok := evdev.KEYFromString["KEY_"+upper]; ok {
		return KeySlot{code}, nil
	}

	return nil, fmt.Errorf("unknown key name")
}
