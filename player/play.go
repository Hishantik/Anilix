package player

import (
	"fmt"
	"os/exec"
	"strings"
)

type Player struct {
	Name string
}

type Options struct {
	Title     string
	Subtitles []string
	Referrer  string
}

var (
	Mpv  = &Player{Name: "mpv"}
	Vlc  = &Player{Name: "vlc"}
	Iina = &Player{Name: "iina"}
)

func (p *Player) String() string {
	return p.Name
}

func (p *Player) Launch(url string, opts Options) error {
	var args []string

	switch p.Name {
	case "mpv":
		args = p.mpvArgs(url, opts)
	case "vlc":
		args = p.vlcArgs(url, opts)
	case "iina":
		args = p.iinaArgs(url, opts)
	default:
		return fmt.Errorf("unknown player: %s", p.Name)
	}

	return exec.Command(p.Name, args...).Start()
}

func (p *Player) mpvArgs(url string, opts Options) []string {
	args := []string{}

	if opts.Title != "" {
		args = append(args, "--force-media-title="+opts.Title)
	}

	if opts.Subtitles != nil {
		for _, sub := range opts.Subtitles {
			args = append(args, "--sub-file="+sub)
		}
	}

	if opts.Referrer != "" {
		args = append(args, "--referrer="+opts.Referrer)
	}

	args = append(args, url)
	return args
}

func (p *Player) vlcArgs(url string, opts Options) []string {
	args := []string{}

	if opts.Referrer != "" {
		args = append(args, "--http-referrer="+opts.Referrer)
	}

	if opts.Title != "" {
		args = append(args, "--meta-title="+opts.Title)
	}

	args = append(args, url)
	return args
}

func (p *Player) iinaArgs(url string, opts Options) []string {
	args := []string{"--no-playlist"}

	if opts.Title != "" {
		args = append(args, "--force-media-title="+opts.Title)
	}

	args = append(args, url)
	return args
}

// FromString returns a Player from name string
func FromString(name string) *Player {
	switch strings.ToLower(name) {
	case "mpv":
		return Mpv
	case "vlc":
		return Vlc
	case "iina":
		return Iina
	default:
		return Mpv // default to mpv
	}
}

// IsAvailable checks if a player is installed
func IsAvailable(name string) bool {
	cmd := exec.Command(name, "--version")
	return cmd.Run() == nil
}