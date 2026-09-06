# VoiceCmd Systemd Service Setup

To run `voicecmd` automatically in the background as a systemd user service:

### 1. Enable Input Group Permissions
Evdev requires read permissions on `/dev/input/event*`:
```bash
sudo usermod -aG input $USER
```
> **Note:** Log out and log back in (or restart) for group membership to take effect.

### 2. Install Binary and Configuration
```bash
make install
```
This copies:
- `voicecmd` -> `~/.local/bin/voicecmd`
- `config/config.yaml` -> `~/.config/voicecmd/config.yaml`

### 3. Install and Enable the Service
```bash
mkdir -p ~/.config/systemd/user
cp systemd/voicecmd.service ~/.config/systemd/user/
systemctl --user daemon-reload
systemctl --user enable --now voicecmd.service
```

### 4. Check Status and Logs
```bash
# Check service status
systemctl --user status voicecmd.service

# View live logs
journalctl --user -u voicecmd.service -f
```
