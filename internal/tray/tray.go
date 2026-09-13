package tray

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"fyne.io/systray"
	"github.com/godbus/dbus/v5"

	"voicecmd/internal/config"
)

// IsDBusAvailable checks whether a D-Bus session bus is currently reachable.
func IsDBusAvailable() bool {
	conn, err := dbus.SessionBus()
	if err != nil {
		return false
	}
	_ = conn
	return true
}

// Manager manages the system tray icon, state, and context menu.
type Manager struct {
	mu         sync.Mutex
	cfg        *config.Config
	configPath string
	state      State

	isReady   bool
	readyChan chan struct{}
	quitOnce  sync.Once

	// Context menu items
	mStatus       *systray.MenuItem
	mHotkey       *systray.MenuItem
	mSTT          *systray.MenuItem
	mSTTDetails   *systray.MenuItem
	mAudio        *systray.MenuItem
	mCommands     *systray.MenuItem
	commandItems  []*systray.MenuItem
	mConfigFile   *systray.MenuItem
	mOpenConfig   *systray.MenuItem
	mReloadConfig *systray.MenuItem
	mQuit         *systray.MenuItem

	onReload func() (string, error)
	onQuit   func()
}

// NewManager creates a new tray manager instance.
func NewManager(
	cfg *config.Config,
	configPath string,
	onReload func() (string, error),
	onQuit func(),
) *Manager {
	return &Manager{
		cfg:        cfg,
		configPath: configPath,
		state:      StateIdle,
		readyChan:  make(chan struct{}),
		onReload:   onReload,
		onQuit:     onQuit,
	}
}

// Start launches the system tray event loop. Blocks until the tray is initialized or times out.
func (m *Manager) Start() error {
	if !IsDBusAvailable() {
		return fmt.Errorf("D-Bus session bus not available")
	}

	go func() {
		systray.Run(m.onReady, m.onExit)
	}()

	select {
	case <-m.readyChan:
		return nil
	case <-time.After(3 * time.Second):
		return fmt.Errorf("timeout waiting for system tray to initialize")
	}
}

// onReady is called when the systray event loop is ready.
func (m *Manager) onReady() {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Initial icon and status
	systray.SetIcon(IconForState(m.state))
	systray.SetTitle("VoiceCmd")
	systray.SetTooltip(m.state.Tooltip(m.cfg.Hotkey.Key))

	// Status indicator item
	m.mStatus = systray.AddMenuItem(m.state.StatusText(), "Current VoiceCmd status")
	m.mStatus.Disable()

	systray.AddSeparator()

	// Display configuration items
	m.mHotkey = systray.AddMenuItem(formatHotkey(m.cfg), "Configured push-to-talk hotkey")
	m.mHotkey.Disable()

	m.mSTT = systray.AddMenuItem(formatSTT(m.cfg), "Whisper model and language")
	m.mSTT.Disable()

	m.mSTTDetails = systray.AddMenuItem(formatSTTDetails(m.cfg), "Whisper inference settings")
	m.mSTTDetails.Disable()

	m.mAudio = systray.AddMenuItem(formatAudio(m.cfg), "Audio input device configuration")
	m.mAudio.Disable()

	m.mCommands = systray.AddMenuItem(formatCommandsTitle(m.cfg), "Loaded voice commands")
	m.populateCommandSubmenuLocked()

	m.mConfigFile = systray.AddMenuItem(formatConfigPath(m.configPath), m.configPath)
	m.mConfigFile.Disable()

	systray.AddSeparator()

	// Action menu items
	m.mOpenConfig = systray.AddMenuItem("Open Configuration File", "Open config file in default editor")
	m.mReloadConfig = systray.AddMenuItem("Reload Configuration", "Reload configuration from disk")

	systray.AddSeparator()

	// Quit action
	m.mQuit = systray.AddMenuItem("Quit VoiceCmd", "Exit VoiceCmd")

	// Start click event handlers
	go m.handleClicks()

	m.isReady = true
	close(m.readyChan)
}

// onExit is called when systray exits.
func (m *Manager) onExit() {
	// Cleanup if needed
}

// handleClicks listens for clicks on interactive menu items.
func (m *Manager) handleClicks() {
	for {
		select {
		case <-m.mOpenConfig.ClickedCh:
			m.mu.Lock()
			path := m.configPath
			m.mu.Unlock()

			if err := OpenConfigFile(path); err != nil {
				log.Printf("[TRAY] Failed to open config file: %v", err)
				sendNotification("VoiceCmd Error", fmt.Sprintf("Failed to open config file: %v", err))
			}

		case <-m.mReloadConfig.ClickedCh:
			if m.onReload != nil {
				msg, err := m.onReload()
				if err != nil {
					log.Printf("[TRAY] Config reload error: %v", err)
					sendNotification("VoiceCmd Config Error", fmt.Sprintf("Reload failed: %v", err))
				} else {
					log.Printf("[TRAY] %s", msg)
					sendNotification("VoiceCmd", msg)
				}
			}

		case <-m.mQuit.ClickedCh:
			if m.onQuit != nil {
				m.onQuit()
			}
			m.Quit()
			return
		}
	}
}

// SetState updates the tray icon and status display to reflect the new state.
func (m *Manager) SetState(state State) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.state = state
	if !m.isReady {
		return
	}

	systray.SetIcon(IconForState(state))
	var hotkey string
	if m.cfg != nil {
		hotkey = m.cfg.Hotkey.Key
	}
	systray.SetTooltip(state.Tooltip(hotkey))
	if m.mStatus != nil {
		m.mStatus.SetTitle(state.StatusText())
	}
}

// UpdateConfig updates the tray menu items to reflect new configuration values.
func (m *Manager) UpdateConfig(newCfg *config.Config, newPath string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.cfg = newCfg
	m.configPath = newPath

	if !m.isReady {
		return
	}

	systray.SetTooltip(m.state.Tooltip(newCfg.Hotkey.Key))

	if m.mHotkey != nil {
		m.mHotkey.SetTitle(formatHotkey(newCfg))
	}
	if m.mSTT != nil {
		m.mSTT.SetTitle(formatSTT(newCfg))
	}
	if m.mSTTDetails != nil {
		m.mSTTDetails.SetTitle(formatSTTDetails(newCfg))
	}
	if m.mAudio != nil {
		m.mAudio.SetTitle(formatAudio(newCfg))
	}
	if m.mConfigFile != nil {
		m.mConfigFile.SetTitle(formatConfigPath(newPath))
		m.mConfigFile.SetTooltip(newPath)
	}
	if m.mCommands != nil {
		m.mCommands.SetTitle(formatCommandsTitle(newCfg))
		m.populateCommandSubmenuLocked()
	}
}

// Quit terminates the tray icon.
func (m *Manager) Quit() {
	m.quitOnce.Do(func() {
		systray.Quit()
	})
}

func (m *Manager) populateCommandSubmenuLocked() {
	if m.mCommands == nil {
		return
	}

	for _, item := range m.commandItems {
		item.Remove()
	}
	m.commandItems = nil

	if m.cfg == nil || len(m.cfg.Commands) == 0 {
		sub := m.mCommands.AddSubMenuItem("No commands configured", "Add commands in configuration file")
		sub.Disable()
		m.commandItems = append(m.commandItems, sub)
		return
	}

	var names []string
	for name := range m.cfg.Commands {
		names = append(names, name)
	}
	sort.Strings(names)

	for _, name := range names {
		cmd := m.cfg.Commands[name]
		phraseCount := len(cmd.Phrases)
		var label string
		if phraseCount == 1 {
			label = fmt.Sprintf("%s (%s)", name, cmd.Phrases[0])
		} else if phraseCount > 1 {
			label = fmt.Sprintf("%s (%d phrases)", name, phraseCount)
		} else {
			label = name
		}

		tooltip := fmt.Sprintf("Action: %s\nProgram: %s %v\nPhrases: %s",
			name, cmd.Command.Program, cmd.Command.Args, strings.Join(cmd.Phrases, ", "))
		sub := m.mCommands.AddSubMenuItem(label, tooltip)
		sub.Disable()
		m.commandItems = append(m.commandItems, sub)
	}
}

func formatHotkey(cfg *config.Config) string {
	if cfg == nil || cfg.Hotkey.Key == "" {
		return "Hotkey: ctrl+space"
	}
	return fmt.Sprintf("Hotkey: %s", cfg.Hotkey.Key)
}

func formatSTT(cfg *config.Config) string {
	if cfg == nil {
		return "STT: whisper.cpp"
	}
	model := filepath.Base(cfg.STT.ModelPath)
	if model == "." || model == "" {
		model = "default"
	}
	lang := cfg.STT.Language
	if lang == "" {
		lang = "en"
	}
	return fmt.Sprintf("STT Model: %s (%s)", model, lang)
}

func formatSTTDetails(cfg *config.Config) string {
	if cfg == nil {
		return "STT Engine: 4 threads"
	}
	return fmt.Sprintf("STT Engine: %d threads | %.2f thresh", cfg.STT.Threads, cfg.STT.Threshold)
}

func formatAudio(cfg *config.Config) string {
	if cfg == nil {
		return "Audio: 16000Hz (mono)"
	}
	dev := cfg.Audio.Device
	if dev == "" {
		dev = "Default Mic"
	}
	if len(dev) > 20 {
		dev = dev[:17] + "..."
	}
	return fmt.Sprintf("Audio: %s (%dHz)", dev, cfg.Audio.SampleRate)
}

func formatCommandsTitle(cfg *config.Config) string {
	if cfg == nil {
		return "Commands: 0 loaded"
	}
	return fmt.Sprintf("Commands: %d loaded", len(cfg.Commands))
}

func formatConfigPath(path string) string {
	if path == "" {
		return "Config: (default)"
	}
	home, _ := os.UserHomeDir()
	display := path
	if home != "" && strings.HasPrefix(path, home) {
		display = "~" + path[len(home):]
	}
	if len(display) > 35 {
		display = "..." + display[len(display)-32:]
	}
	return fmt.Sprintf("Config: %s", display)
}

func sendNotification(title, message string) {
	if p, err := exec.LookPath("notify-send"); err == nil {
		_ = exec.Command(p, "-a", "VoiceCmd", title, message).Start()
	}
}
