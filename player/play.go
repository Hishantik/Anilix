package player

import (
	"fmt"
	"os/exec"
	"strings"
	"time"
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
	Mpv        = &Player{Name: "mpv"}
	Vlc        = &Player{Name: "vlc"}
	Iina       = &Player{Name: "iina"}
	MpvAndroid = &Player{Name: "mpv-android"}
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
	case "mpv-android":
		args = p.mpvAndroidArgs(url, opts)
	case "vlc-android":
		args = p.vlcAndroidArgs(url, opts)
	default:
		return fmt.Errorf("unknown player: %s", p.Name)
	}

	if p.Name == "mpv-android" || p.Name == "vlc-android" {
		// Start local proxy so the player can fetch via localhost
		localURL, stop, err := StartProxy(url, opts.Referrer)
		if err != nil {
			return fmt.Errorf("proxy start failed: %w", err)
		}
		// Keep proxy alive in background for 30 min
		go func() {
			time.Sleep(30 * time.Minute)
			stop()
		}()

		// Replace -d URL arg with local proxy URL
		for i, a := range args {
			if a == "-d" && i+1 < len(args) {
				args[i+1] = localURL
				break
			}
		}

		cmd := exec.Command("am", args...)
		out, err := cmd.CombinedOutput()
		if err != nil {
			return fmt.Errorf("am start failed: %s %w", string(out), err)
		}
		_ = out
		return nil
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

func (p *Player) mpvAndroidArgs(url string, opts Options) []string {
	args := []string{
		"start",
		"-a", "android.intent.action.VIEW",
		"-t", "video/*",
		"-d", url,
		"-n", "is.xyz.mpv/.MPVActivity",
	}
	if opts.Title != "" {
		args = append(args, "--es", "title", opts.Title)
	}
	return args
}

func (p *Player) vlcAndroidArgs(url string, opts Options) []string {
	mimeType := "video/*"
	lower := strings.ToLower(url)
	if strings.HasSuffix(lower, ".m3u8") || strings.Contains(lower, ".m3u8?") {
		mimeType = "application/x-mpegURL"
	}
	args := []string{
		"start",
		"-a", "android.intent.action.VIEW",
		"-t", mimeType,
		"-d", url,
		"-n", "org.videolan.vlc/.gui.video.VideoPlayerActivity",
	}
	if opts.Title != "" {
		args = append(args, "--es", "title", opts.Title)
	}
	if opts.Referrer != "" {
		args = append(args, "--es", "http-referrer", opts.Referrer)
	}
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
	case "mpv-android":
		return MpvAndroid
	case "vlc-android":
		return VlcAndroid
	default:
		return Mpv // default to mpv
	}
}