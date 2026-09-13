package hotkey

import (
	"context"
	"fmt"
	"os"
	"strings"
	"sync"

	"github.com/holoplot/go-evdev"
)

// KeyboardInfo describes an identified input device.
type KeyboardInfo struct {
	Path string
	Name string
}

// Listener listens to keyboard events and notifies callbacks on combo press and release.
type Listener struct {
	combo      KeyCombo
	devicePath string

	mu          sync.Mutex
	devices     []*evdev.InputDevice
	pressedKeys map[evdev.EvCode]bool
	isTriggered bool

	onPress   func()
	onRelease func()
}

// NewListener creates a listener for the specified combination.
// If devicePath is empty, all keyboards will be listened to.
func NewListener(comboStr, devicePath string, onPress, onRelease func()) (*Listener, error) {
	combo, err := ParseCombo(comboStr)
	if err != nil {
		return nil, fmt.Errorf("failed to parse hotkey combination: %w", err)
	}

	return &Listener{
		combo:       combo,
		devicePath:  devicePath,
		pressedKeys: make(map[evdev.EvCode]bool),
		onPress:     onPress,
		onRelease:   onRelease,
	}, nil
}

// UpdateCombo updates the listening key combination at runtime.
func (l *Listener) UpdateCombo(comboStr string) error {
	combo, err := ParseCombo(comboStr)
	if err != nil {
		return fmt.Errorf("failed to parse hotkey combination: %w", err)
	}

	l.mu.Lock()
	l.combo = combo
	l.isTriggered = false
	l.pressedKeys = make(map[evdev.EvCode]bool)
	l.mu.Unlock()

	return nil
}

// Start begins listening for the hotkey combination. Blocks until ctx is cancelled or a fatal error occurs.
func (l *Listener) Start(ctx context.Context) error {
	devices, err := l.openDevices()
	if err != nil {
		return err
	}
	defer l.closeDevices()

	l.devices = devices

	events := make(chan *evdev.InputEvent, 64)
	errChan := make(chan error, len(devices))
	var wg sync.WaitGroup

	for _, dev := range devices {
		wg.Add(1)
		go func(d *evdev.InputDevice) {
			defer wg.Done()
			for {
				ev, rErr := d.ReadOne()
				if rErr != nil {
					select {
					case <-ctx.Done():
						return
					default:
						errChan <- rErr
						return
					}
				}
				select {
				case <-ctx.Done():
					return
				case events <- ev:
				}
			}
		}(dev)
	}

	for {
		select {
		case <-ctx.Done():
			l.closeDevices()
			wg.Wait()
			return nil

		case err := <-errChan:
			return fmt.Errorf("device read error: %w", err)

		case ev := <-events:
			if ev.Type != evdev.EV_KEY {
				continue
			}

			l.handleKeyEvent(ev)
		}
	}
}

func (l *Listener) handleKeyEvent(ev *evdev.InputEvent) {
	l.mu.Lock()
	defer l.mu.Unlock()

	code := ev.Code
	val := ev.Value

	switch val {
	case 1: // Key Down
		l.pressedKeys[code] = true
		if !l.isTriggered && l.combo.IsActive(l.pressedKeys) {
			l.isTriggered = true
			if l.onPress != nil {
				go l.onPress()
			}
		}

	case 0: // Key Up
		delete(l.pressedKeys, code)
		if l.isTriggered && !l.combo.IsActive(l.pressedKeys) {
			l.isTriggered = false
			if l.onRelease != nil {
				go l.onRelease()
			}
		}

	case 2: // Key Repeat
		// Keep state as pressed
		l.pressedKeys[code] = true
	}
}

func (l *Listener) openDevices() ([]*evdev.InputDevice, error) {
	if l.devicePath != "" {
		dev, err := evdev.Open(l.devicePath)
		if err != nil {
			return nil, checkPermError(err, l.devicePath)
		}
		return []*evdev.InputDevice{dev}, nil
	}

	keyboards, err := FindKeyboards()
	if err != nil {
		return nil, err
	}

	if len(keyboards) == 0 {
		return nil, fmt.Errorf("no keyboard input devices found. If this is a permission issue, run 'sudo usermod -aG input $USER' or execute with sudo")
	}

	var opened []*evdev.InputDevice
	for _, kb := range keyboards {
		dev, err := evdev.Open(kb.Path)
		if err != nil {
			continue
		}
		opened = append(opened, dev)
	}

	if len(opened) == 0 {
		return nil, fmt.Errorf("unable to open any keyboard input devices: permission denied. Add your user to the 'input' group: 'sudo usermod -aG input $USER' or run with sudo")
	}

	return opened, nil
}

func (l *Listener) closeDevices() {
	l.mu.Lock()
	defer l.mu.Unlock()

	for _, dev := range l.devices {
		dev.Close()
	}
	l.devices = nil
}

// FindKeyboards scans /dev/input for devices that have keyboard capabilities.
func FindKeyboards() ([]KeyboardInfo, error) {
	paths, err := evdev.ListDevicePaths()
	if err != nil {
		return nil, checkPermError(err, "/dev/input")
	}

	var result []KeyboardInfo
	permDeniedCount := 0

	for _, p := range paths {
		dev, err := evdev.Open(p.Path)
		if err != nil {
			if os.IsPermission(err) {
				permDeniedCount++
			}
			continue
		}

		name, _ := dev.Name()
		keys := dev.CapableEvents(evdev.EV_KEY)
		dev.Close()

		// A standard keyboard must have basic alphanumeric keys (e.g. KEY_A, KEY_SPACE, KEY_ENTER)
		hasA := false
		hasSpace := false
		hasEnter := false

		for _, k := range keys {
			if k == evdev.KEY_A {
				hasA = true
			}
			if k == evdev.KEY_SPACE {
				hasSpace = true
			}
			if k == evdev.KEY_ENTER {
				hasEnter = true
			}
		}

		if hasA && hasSpace && hasEnter {
			result = append(result, KeyboardInfo{
				Path: p.Path,
				Name: name,
			})
		}
	}

	if len(result) == 0 && permDeniedCount > 0 {
		return nil, fmt.Errorf("permission denied accessing /dev/input devices (%d devices inaccessible). Add user to input group: 'sudo usermod -aG input $USER' or run with sudo", permDeniedCount)
	}

	return result, nil
}

func checkPermError(err error, path string) error {
	if os.IsPermission(err) || strings.Contains(err.Error(), "permission denied") {
		return fmt.Errorf("permission denied opening %s. Run 'sudo usermod -aG input $USER' (relogin required) or run with sudo: %w", path, err)
	}
	return err
}
