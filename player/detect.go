package player

import (
	"os/exec"
	"runtime"
)

type Detector struct{}

// IsAndroid returns true if running on an Android environment.
func IsAndroid() bool {
	return runtime.GOOS == "android"
}

func (d *Detector) Detect() []string {
	if IsAndroid() {
		if isMpvAndroidInstalled() {
			return []string{"mpv-android"}
		}
		return nil
	}

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
	if IsAndroid() {
		if isMpvAndroidInstalled() {
			return MpvAndroid
		}
		return MpvAndroid // fallback, will fail on Launch
	}

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

func isMpvAndroidInstalled() bool {
	cmd := exec.Command("pm", "list", "packages", "is.xyz.mpv")
	out, err := cmd.Output()
	if err != nil {
		return false
	}
	return len(out) > 0
}