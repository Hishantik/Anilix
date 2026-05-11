package player

import (
	"os/exec"
	"runtime"
)

type Detector struct{}

func (d *Detector) Detect() []string {
	players := []string{"mpv", "vlc"}
	if runtime.GOOS == "darwin" {
		players = append(players, "iina")
	}

	var installed []string
	for _, name := range players {
		if IsAvailable(name) {
			installed = append(installed, name)
		}
	}

	return installed
}

func (d *Detector) Preferred() *Player {
	order := []string{"mpv", "vlc"}
	if runtime.GOOS == "darwin" {
		order = []string{"mpv", "vlc", "iina"}
	}

	for _, name := range order {
		if IsAvailable(name) {
			return FromString(name)
		}
	}

	return Mpv
}

func IsAvailable(name string) bool {
	cmd := exec.Command(name, "--version")
	return cmd.Run() == nil
}