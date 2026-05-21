package player

import (
	"fmt"
	"net"
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

		// Wait for proxy to be ready
		proxyAddr := strings.TrimPrefix(strings.TrimSuffix(localURL, "/video"), "http://")
		if err := waitForListen(proxyAddr, 2*time.Second); err != nil {
			stop()
			return fmt.Errorf("proxy not ready: %w", err)
		}

		// Replace -d URL arg with local proxy URL
		for i, a := range args {
			if a == "-d" && i+1 < len(args) {
				args[i+1] = localURL
				break
			}
		}

		// Try am start with -n flag first (works on real Android am)
		cmd := exec.Command("am", args...)
		if _, err := cmd.CombinedOutput(); err != nil {
			// Fallback: try without -n (Termux am wrapper doesn't support -n)
			argsNoN := removeFlag(args, "-n", 1)
			cmd2 := exec.Command("am", argsNoN...)
			out2, err2 := cmd2.CombinedOutput()
			if err2 != nil {
				return fmt.Errorf("am start failed (both attempts): %s %w", string(out2), err2)
			}
		}
		return nil
	}
	return exec.Command(p.Name, args...).Start()
}

// removeFlag removes a flag and its N following values from args.
func removeFlag(args []string, flag string, nValues int) []string {
	var result []string
	for i := 0; i < len(args); i++ {
		if args[i] == flag {
			i += nValues // skip flag and its values
			continue
		}
		result = append(result, args[i])
	}
	return result
}

// waitForListen polls until the given address accepts a TCP connection.
func waitForListen(addr string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		conn, err := net.DialTimeout("tcp", addr, 200*time.Millisecond)
		if err == nil {
			conn.Close()
			return nil
		}
		time.Sleep(100 * time.Millisecond)
	}
	return fmt.Errorf("timed out waiting for %s", addr)
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
	// Termux am wrapper supports -a, -d, -t, --es, --user but NOT -n.
	// Use -a with component specified via -d intent data or package manager.
	// We try -n first (works on real am), fallback handled in Launch().
	args := []string{
		"start",
		"--user", "0",
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
		"--user", "0",
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