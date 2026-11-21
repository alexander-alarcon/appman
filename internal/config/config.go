package config

import (
	"os"
	"path/filepath"
)

type Config struct {
	General GeneralConfig
	Scanner ScannerConfig
	Desktop DesktopConfig
}

type GeneralConfig struct {
	AppImageDir string
	DesktopDir  string
	IconDir     string
}

type ScannerConfig struct {
	Recursive    bool
	ExtractIcons bool
}

type DesktopConfig struct {
	Categories    []string
	StartupNotify bool
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

// Load returns default configuration. The MVP uses flags only (no config file).
func Load() (Config, error) {
	return Default(), nil
}
