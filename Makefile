BINARY := voicecmd
PKG := ./cmd/voicecmd

.PHONY: all build run ptt test clean list install setup-permissions

all: build

build:
	go build -ldflags="-s -w" -o $(BINARY) $(PKG)

run: build
	./$(BINARY) -config config/config.yaml

ptt: build
	./$(BINARY) -config config/config.yaml -ptt

test:
	go test -v ./...

list: build
	./$(BINARY) -list-devices

install: build
	mkdir -p $(HOME)/.local/bin $(HOME)/.config/voicecmd
	cp $(BINARY) $(HOME)/.local/bin/$(BINARY)
	@if [ ! -f $(HOME)/.config/voicecmd/config.yaml ]; then \
		cp config/config.yaml $(HOME)/.config/voicecmd/config.yaml; \
		echo "Copied default config to ~/.config/voicecmd/config.yaml"; \
	fi
	@echo "Installed $(BINARY) to $(HOME)/.local/bin/$(BINARY)"

setup-permissions:
	@echo "To allow VoiceCmd to monitor keyboard hotkeys without sudo, run:"
	@echo "  sudo usermod -aG input $$USER"
	@echo "Then log out and log back in."

clean:
	rm -f $(BINARY) test.wav /tmp/voicecmd_recording.wav
