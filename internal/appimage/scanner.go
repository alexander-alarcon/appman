package appimage

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type Image struct {
	Path      string
	Filename  string
	NameGuess string
	Size      int64
	ModTime   time.Time
	Mode      fs.FileMode
}

type ScanResult struct {
	Images []Image
	Errors []error
}

func Scan(dir string) (ScanResult, error) {
	dir = expandHome(dir)
	info, err := os.Stat(dir)
	if err != nil {
		return ScanResult{}, fmt.Errorf("scan dir: %w", err)
	}
	if !info.IsDir() {
		return ScanResult{}, fmt.Errorf("not a directory: %s", dir)
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		return ScanResult{}, fmt.Errorf("read dir: %w", err)
	}

	res := ScanResult{}
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if !strings.HasSuffix(strings.ToLower(name), ".appimage") {
			continue
		}
		full := filepath.Join(dir, name)
		fi, err := e.Info()
		if err != nil {
			res.Errors = append(res.Errors, fmt.Errorf("%s: %w", full, err))
			continue
		}
		res.Images = append(res.Images, Image{
			Path:      full,
			Filename:  name,
			NameGuess: guessName(name),
			Size:      fi.Size(),
			ModTime:   fi.ModTime(),
			Mode:      fi.Mode(),
		})
	}
	return res, nil
}

func guessName(filename string) string {
	base := strings.TrimSuffix(filename, filepath.Ext(filename))
	base = strings.TrimSuffix(base, ".AppImage")
	base = strings.TrimSuffix(base, ".appimage")
	// crude: drop trailing -x.y.z or version-like suffix
	parts := strings.Split(base, "-")
	if len(parts) > 1 {
		last := parts[len(parts)-1]
		if len(last) > 0 && (last[0] >= '0' && last[0] <= '9') {
			base = strings.Join(parts[:len(parts)-1], "-")
		}
	}
	if base == "" {
		return filename
	}
	return base
}

func expandHome(p string) string {
	if p == "~" {
		if h, err := os.UserHomeDir(); err == nil {
			return h
		}
	}
	if strings.HasPrefix(p, "~/") {
		if h, err := os.UserHomeDir(); err == nil {
			return filepath.Join(h, p[2:])
		}
	}
	return p
}
