package config

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/viper"
)

const (
	AppName = "anilix"
)

var v = viper.New()

func Setup() error {
	setName()
	setFs()
	setEnvs()
	setDefaults()
	setPaths()

	if err := v.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			return err
		}
	}

	return nil
}

func setName() {
	v.SetConfigName(AppName)
	v.SetConfigType("toml")
}

func setFs() {
	v.SetFs(nil)
}

func setEnvs() {
	v.SetEnvPrefix(AppName)
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
}

func setDefaults() {
	v.SetDefault("player", "mpv")
	v.SetDefault("quality", "480p")
	v.SetDefault("source", "")
	v.SetDefault("history.enabled", true)
	v.SetDefault("aniskip.enabled", true)
}

func setPaths() {
	home, err := os.UserHomeDir()
	if err != nil {
		home = "~"
	}

	configDir := filepath.Join(home, "."+AppName)
	v.AddConfigPath(configDir)

	_ = os.MkdirAll(configDir, 0755)
}

func Get(key string) interface{} {
	return v.Get(key)
}

func GetString(key string) string {
	return v.GetString(key)
}

func GetBool(key string) bool {
	return v.GetBool(key)
}

func Set(key string, value interface{}) {
	v.Set(key, value)
	Save()
}

// Save persists the current config to disk.
func Save() {
	home, err := os.UserHomeDir()
	if err != nil {
		return
	}
	dir := filepath.Join(home, "."+AppName)
	_ = os.MkdirAll(dir, 0755)
	path := filepath.Join(dir, AppName+".toml")
	_ = v.WriteConfigAs(path)
}

// HistoryPath returns the path to the history file
func HistoryPath() string {
	home, _ := os.UserHomeDir()
	if home == "" {
		home = "~"
	}
	return filepath.Join(home, "."+AppName, "history.json")
}