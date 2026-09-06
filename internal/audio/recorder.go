package audio

import (
	"fmt"
	"os"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/gordonklaus/portaudio"
)

var (
	paInitOnce sync.Once
	paInitErr  error
)

// Init initializes PortAudio globally. Safe to call multiple times.
func Init() error {
	paInitOnce.Do(func() {
		devNull, err := os.OpenFile(os.DevNull, os.O_WRONLY, 0)
		var oldStderr int
		if err == nil {
			oldStderr, _ = syscall.Dup(int(os.Stderr.Fd()))
			_ = syscall.Dup2(int(devNull.Fd()), int(os.Stderr.Fd()))
		}

		paInitErr = portaudio.Initialize()

		if err == nil {
			_ = syscall.Dup2(oldStderr, int(os.Stderr.Fd()))
			_ = syscall.Close(oldStderr)
			_ = devNull.Close()
		}
	})
	return paInitErr
}

// Terminate terminates PortAudio. Call on application exit.
func Terminate() error {
	return portaudio.Terminate()
}

// ListDevices returns names of all available input devices.
func ListDevices() ([]string, error) {
	if err := Init(); err != nil {
		return nil, fmt.Errorf("failed to init PortAudio: %w", err)
	}

	devices, err := portaudio.Devices()
	if err != nil {
		return nil, err
	}

	var inputs []string
	for _, dev := range devices {
		if dev.MaxInputChannels > 0 {
			inputs = append(inputs, fmt.Sprintf("%s (inputs=%d)", dev.Name, dev.MaxInputChannels))
		}
	}
	return inputs, nil
}

// Recorder captures microphone audio into sample buffers.
type Recorder struct {
	deviceName string
	sampleRate int
	channels   int
	bufferSize int

	mu          sync.Mutex
	isRecording bool
	stopChan    chan struct{}
	doneChan    chan struct{}
	samples     []float32
	lastErr     error
	startTime   time.Time
}

// NewRecorder creates a new audio recorder instance.
func NewRecorder(deviceName string, sampleRate, channels int) *Recorder {
	if sampleRate <= 0 {
		sampleRate = 16000
	}
	if channels <= 0 {
		channels = 1
	}
	return &Recorder{
		deviceName: deviceName,
		sampleRate: sampleRate,
		channels:   channels,
		bufferSize: 1600, // 100ms at 16kHz
	}
}

// Start begins recording audio asynchronously.
func (r *Recorder) Start() error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.isRecording {
		return nil // already recording
	}

	if err := Init(); err != nil {
		return fmt.Errorf("failed to init PortAudio: %w", err)
	}

	buffer := make([]float32, r.bufferSize)
	var stream *portaudio.Stream
	var err error

	if r.deviceName != "" {
		devices, devErr := portaudio.Devices()
		if devErr != nil {
			return fmt.Errorf("failed to query devices: %w", devErr)
		}
		var targetDev *portaudio.DeviceInfo
		for _, dev := range devices {
			if dev.MaxInputChannels > 0 && strings.Contains(strings.ToLower(dev.Name), strings.ToLower(r.deviceName)) {
				targetDev = dev
				break
			}
		}
		if targetDev == nil {
			return fmt.Errorf("audio device matching %q not found", r.deviceName)
		}
		params := portaudio.LowLatencyParameters(targetDev, nil)
		params.Input.Channels = r.channels
		params.SampleRate = float64(r.sampleRate)
		params.FramesPerBuffer = r.bufferSize
		stream, err = portaudio.OpenStream(params, buffer)
	} else {
		stream, err = portaudio.OpenDefaultStream(r.channels, 0, float64(r.sampleRate), r.bufferSize, buffer)
	}

	if err != nil {
		return fmt.Errorf("failed to open portaudio stream: %w", err)
	}

	if err := stream.Start(); err != nil {
		stream.Close()
		return fmt.Errorf("failed to start stream: %w", err)
	}

	r.isRecording = true
	r.samples = make([]float32, 0, r.sampleRate*5) // pre-allocate for 5 seconds
	r.stopChan = make(chan struct{})
	r.doneChan = make(chan struct{})
	r.lastErr = nil
	r.startTime = time.Now()

	go r.recordLoop(stream, buffer)
	return nil
}

func (r *Recorder) recordLoop(stream *portaudio.Stream, buffer []float32) {
	defer close(r.doneChan)
	defer stream.Close()
	defer stream.Stop()

	for {
		select {
		case <-r.stopChan:
			return
		default:
			if err := stream.Read(); err != nil {
				r.mu.Lock()
				r.lastErr = err
				r.mu.Unlock()
				return
			}
			r.mu.Lock()
			r.samples = append(r.samples, buffer...)
			r.mu.Unlock()
		}
	}
}

// Stop stops recording and returns recorded audio samples.
func (r *Recorder) Stop() ([]float32, time.Duration, error) {
	r.mu.Lock()
	if !r.isRecording {
		r.mu.Unlock()
		return nil, 0, nil
	}

	r.isRecording = false
	close(r.stopChan)
	r.mu.Unlock()

	// Wait for recording loop to cleanly finish
	<-r.doneChan

	r.mu.Lock()
	defer r.mu.Unlock()

	duration := time.Since(r.startTime)
	samplesCopy := make([]float32, len(r.samples))
	copy(samplesCopy, r.samples)

	return samplesCopy, duration, r.lastErr
}

// IsRecording returns true if currently recording.
func (r *Recorder) IsRecording() bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.isRecording
}
