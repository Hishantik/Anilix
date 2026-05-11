package player

import (
	"testing"
)

func TestDetectorDetect(t *testing.T) {
	d := &Detector{}
	players := d.Detect()

	if len(players) == 0 {
		t.Log("No players detected (expected in test environment)")
	}

	for _, name := range players {
		if name != "mpv" && name != "vlc" && name != "iina" {
			t.Errorf("Unexpected player detected: %s", name)
		}
	}
}

func TestDetectorPreferred(t *testing.T) {
	d := &Detector{}
	preferred := d.Preferred()

	if preferred == nil {
		t.Error("Preferred() returned nil")
	}

	validNames := map[string]bool{"mpv": true, "vlc": true, "iina": true}
	if !validNames[preferred.Name] {
		t.Errorf("Preferred() returned invalid player: %s", preferred.Name)
	}
}

func TestSyncplayNewSyncplay(t *testing.T) {
	sp := NewSyncplay(SyncplayOptions{
		Room:     "test-room",
		Password: "test-pass",
		Player:   Vlc,
	})

	if sp.Room != "test-room" {
		t.Errorf("Room = %v, want test-room", sp.Room)
	}
	if sp.Password != "test-pass" {
		t.Errorf("Password = %v, want test-pass", sp.Password)
	}
	if sp.Player != Vlc {
		t.Errorf("Player = %v, want Vlc", sp.Player)
	}
}

func TestSyncplayNewSyncplayDefaults(t *testing.T) {
	sp := NewSyncplay(SyncplayOptions{
		Room: "test-room",
	})

	if sp.Player != Mpv {
		t.Errorf("Player default = %v, want Mpv", sp.Player)
	}
}

func TestSyncplayLaunchNotAvailable(t *testing.T) {
	sp := &Syncplay{
		Room:   "test-room",
		Player: Mpv,
	}

	err := sp.Launch("https://example.com/video.m3u8", "Test Anime")
	if err == nil {
		t.Error("Expected error when syncplay not available")
	}
}