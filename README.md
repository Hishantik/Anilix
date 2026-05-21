# Anilix

A terminal-based anime streaming client written in Go. Search anime, browse episodes, and stream directly from your terminal.

## Features

- Interactive TUI for searching and selecting anime
- Multi-provider pipeline (Jikan + AllAnime) for metadata and streams
- Automatic stream extraction from multiple hosts
- Local HTTP proxy for seamless Android playback
- Persistent MAL-to-AllAnime ID caching via SQLite

## Supported Platforms

| Platform | Supported Players |
|----------|-------------------|
| Linux | mpv, vlc |
| macOS | mpv, vlc, iina |
| Windows | mpv, vlc |
| Android | mpv-android, vlc-android |

## Installation

### Quick Install (Recommended)

```bash
curl -fsSL https://raw.githubusercontent.com/hishantik/anilix/main/install.sh | sh
```

This automatically detects your platform (Linux, macOS, Termux) and installs the latest release binary.

**Windows (PowerShell):**

```powershell
curl -fsSL https://raw.githubusercontent.com/hishantik/anilix/main/install.bat -o install.bat && .\install.bat
```

### Go Install

```bash
go install github.com/hishantik/anilix@latest
```

### Build from Source

```bash
git clone https://github.com/hishantik/anilix.git
cd anilix
go build -o anilix .
```

> **Note:** On PRoot environments (e.g., Termux), build from `/tmp` to avoid filesystem locking errors:
> ```bash
> mkdir -p /tmp/anilix-build && cp -r . /tmp/anilix-build/ && cd /tmp/anilix-build && go build -o ~/anilix .
> ```

### Manual Download

Download the latest binary for your platform from [GitHub Releases](https://github.com/hishantik/anilix/releases).

## Usage

Launch the interactive TUI:

```bash
anilix tui
```

Or simply run:

```bash
anilix
```

Other commands:

```bash
anilix version   # Print version information
```

### Configuration

Configuration is stored at `~/.anilix/anilix.toml`.

## License

This project is not currently licensed.
