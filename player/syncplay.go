package player

import (
	"fmt"
	"os/exec"
)

type Syncplay struct {
	Room    string
	Password string
	Player  *Player
}

type SyncplayOptions struct {
	Room     string
	Password string
	Player   *Player
}

func NewSyncplay(opts SyncplayOptions) *Syncplay {
	if opts.Player == nil {
		opts.Player = Mpv
	}
	return &Syncplay{
		Room:     opts.Room,
		Password: opts.Password,
		Player:   opts.Player,
	}
}

func (s *Syncplay) Launch(url string, title string) error {
	if !IsAvailable("syncplay") {
		return fmt.Errorf("syncplay is not installed")
	}

	args := []string{
		"--no-gui",
		"--room", s.Room,
	}

	if s.Password != "" {
		args = append(args, "--password", s.Password)
	}

	// Player-specific args for syncplay
	switch s.Player.Name {
	case "mpv":
		args = append(args, url, "--", "--force-media-title="+title)
	case "vlc":
		args = append(args, url)
	default:
		args = append(args, url)
	}

	return exec.Command("syncplay", args...).Start()
}