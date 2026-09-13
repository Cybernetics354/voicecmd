package tray

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

// OpenConfigFile opens the specified configuration file in the user's default desktop editor.
// If xdg-open fails, it attempts to fall back to $VISUAL, $EDITOR, or standard editors.
func OpenConfigFile(filePath string) error {
	if filePath == "" {
		return fmt.Errorf("configuration file path is empty")
	}

	absPath, err := filepath.Abs(filePath)
	if err != nil {
		absPath = filePath
	}

	if _, err := os.Stat(absPath); err != nil {
		return fmt.Errorf("configuration file does not exist: %w", err)
	}

	// 1. Try xdg-open (standard cross-desktop URL/file handler on Linux)
	if xdg, err := exec.LookPath("xdg-open"); err == nil {
		cmd := exec.Command(xdg, absPath)
		if err := cmd.Start(); err == nil {
			return nil
		}
	}

	// 2. Fallback to $VISUAL or $EDITOR
	for _, envVar := range []string{"VISUAL", "EDITOR"} {
		if editor := os.Getenv(envVar); editor != "" {
			cmd := exec.Command(editor, absPath)
			if err := cmd.Start(); err == nil {
				return nil
			}
		}
	}

	// 3. Fallback to common GUI editors
	for _, fallback := range []string{"gedit", "kate", "gnome-text-editor", "mousepad", "code"} {
		if p, err := exec.LookPath(fallback); err == nil {
			cmd := exec.Command(p, absPath)
			if err := cmd.Start(); err == nil {
				return nil
			}
		}
	}

	return fmt.Errorf("no suitable program found to open %q (install xdg-open or set $EDITOR)", absPath)
}
