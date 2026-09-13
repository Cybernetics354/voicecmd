package hotkey

import (
	"bufio"
	"context"
	"fmt"
	"net"
	"os"
	"strings"
	"sync"
)

const DefaultSocketPath = "/tmp/voicecmd.sock"

// IPCServer provides a Unix domain socket server for triggering voice recording.
type IPCServer struct {
	socketPath string
	listener   net.Listener
	onPress    func()
	onRelease  func()
	getStatus  func() string
	onLoad     func(path string) (string, error)
	mu         sync.Mutex
}

// NewIPCServer creates a new IPC socket server.
func NewIPCServer(
	socketPath string,
	onPress, onRelease func(),
	getStatus func() string,
) *IPCServer {
	if socketPath == "" {
		socketPath = DefaultSocketPath
	}
	return &IPCServer{
		socketPath: socketPath,
		onPress:    onPress,
		onRelease:  onRelease,
		getStatus:  getStatus,
	}
}

// SetLoadHandler sets the callback for loading configuration.
func (s *IPCServer) SetLoadHandler(handler func(path string) (string, error)) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.onLoad = handler
}

// Start begins listening on the Unix socket.
func (s *IPCServer) Start(ctx context.Context) error {
	// Remove stale socket if exists
	os.Remove(s.socketPath)

	l, err := net.Listen("unix", s.socketPath)
	if err != nil {
		return fmt.Errorf("failed to listen on unix socket %s: %w", s.socketPath, err)
	}
	s.listener = l

	// Set socket permissions so current user can read/write
	os.Chmod(s.socketPath, 0o660)

	go func() {
		<-ctx.Done()
		s.listener.Close()
		os.Remove(s.socketPath)
	}()

	for {
		conn, err := l.Accept()
		if err != nil {
			select {
			case <-ctx.Done():
				return nil
			default:
				return err
			}
		}

		go s.handleConn(conn)
	}
}

func (s *IPCServer) handleConn(conn net.Conn) {
	defer conn.Close()
	scanner := bufio.NewScanner(conn)
	if err := scanner.Err(); err != nil {
		fmt.Print(err)
		return
	}

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		parts := strings.Fields(line)
		cmd := strings.ToLower(parts[0])

		switch cmd {
		case "start", "press", "down":
			if s.onPress != nil {
				s.onPress()
			}
			fmt.Fprintln(conn, "OK: recording started")
		case "stop", "release", "up":
			if s.onRelease != nil {
				s.onRelease()
			}
			fmt.Fprintln(conn, "OK: recording stopped, transcribing")
		case "status":
			status := "idle"
			if s.getStatus != nil {
				status = s.getStatus()
			}
			fmt.Fprintf(conn, "OK: status=%s\n", status)
		case "load", "reload":
			arg := strings.TrimSpace(line[len(parts[0]):])
			arg = strings.Trim(arg, `"'`)

			s.mu.Lock()
			loadFn := s.onLoad
			s.mu.Unlock()

			if loadFn == nil {
				fmt.Fprintln(conn, "ERR: config loading not supported")
				return
			}

			msg, err := loadFn(arg)
			if err != nil {
				fmt.Fprintf(conn, "ERR: %v\n", err)
				return
			}

			if msg == "" {
				msg = "config loaded"
			}
			fmt.Fprintf(conn, "OK: %s\n", msg)

		default:
			fmt.Fprintf(
				conn,
				"ERR: unknown command %q. Valid commands: start, stop, status, load\n",
				cmd,
			)
		}
	}
}

// SendIPCCommand sends a command to a running voicecmd daemon via Unix socket.
func SendIPCCommand(socketPath, cmd string) (string, error) {
	if socketPath == "" {
		socketPath = DefaultSocketPath
	}

	conn, err := net.Dial("unix", socketPath)
	if err != nil {
		return "", fmt.Errorf("failed to connect to voicecmd daemon at %s: %w", socketPath, err)
	}
	defer conn.Close()

	if _, err := fmt.Fprintf(conn, "%s\n", cmd); err != nil {
		return "", err
	}

	reader := bufio.NewReader(conn)
	resp, err := reader.ReadString('\n')
	if err != nil {
		return "", err
	}

	return strings.TrimSpace(resp), nil
}
