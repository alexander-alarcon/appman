package config

import (
	"os"
	"path/filepath"

	"github.com/BurntSushi/toml"
)

type Config struct {
	General GeneralConfig `toml:"general"`
	Scanner ScannerConfig `toml:"scanner"`
	Desktop DesktopConfig `toml:"desktop"`
}

type GeneralConfig struct {
	AppImageDir string `toml:"appimage_dir"`
	DesktopDir  string `toml:"desktop_dir"`
	IconDir     string `toml:"icon_dir"`
}

type ScannerConfig struct {
	Recursive    bool `toml:"recursive"`
	ExtractIcons bool `toml:"extract_icons"`
}

type DesktopConfig struct {
	Categories    []string `toml:"categories"`
	StartupNotify bool     `toml:"startup_notify"`
}

func Default() Config {
	home, _ := os.UserHomeDir()
	return Config{
		General: GeneralConfig{
			AppImageDir: filepath.Join(home, "Applications", "AppImages"),
			DesktopDir:  filepath.Join(home, ".local", "share", "applications"),
			IconDir:     filepath.Join(home, ".local", "share", "icons", "appman"),
		},
		Scanner: ScannerConfig{
			Recursive:    false,
			ExtractIcons: true,
		},
		Desktop: DesktopConfig{
			Categories:    []string{"Utility"},
			StartupNotify: true,
		},
	}
}

func Load() (Config, error) {
	cfg := Default()
	path := Path()
	if _, err := os.Stat(path); err != nil {
		if os.IsNotExist(err) {
			return cfg, nil
		}
		return cfg, err
	}
	if _, err := toml.DecodeFile(path, &cfg); err != nil {
		return cfg, err
	}
	// expand ~
	cfg.General.AppImageDir = expandHome(cfg.General.AppImageDir)
	cfg.General.DesktopDir = expandHome(cfg.General.DesktopDir)
	cfg.General.IconDir = expandHome(cfg.General.IconDir)
	return cfg, nil
}

func Path() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".config", "appman", "config.toml")
}

func expandHome(p string) string {
	if p == "~" {
		if h, err := os.UserHomeDir(); err == nil {
			return h
		}
	}
	if len(p) >= 2 && p[:2] == "~/" {
		if h, err := os.UserHomeDir(); err == nil {
			return filepath.Join(h, p[2:])
		}
	}
	return p
}
