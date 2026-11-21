package desktop

import (
	"bufio"
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

const (
	ManagedKey        = "X-Appman-Managed"
	ManagedValue      = "true"
	ManagedAppPathKey = "X-Appman-AppImage"
	ManagedIDKey      = "X-Appman-ID"
)

type Entry struct {
	Path string
	// raw key-values for [Desktop Entry]
	Values map[string]string
}

func (e *Entry) Get(k string) string { return e.Values[k] }
func (e *Entry) IsManaged() bool     { return strings.EqualFold(e.Get(ManagedKey), ManagedValue) }

type Spec struct {
	ID            string
	Name          string
	Comment       string
	Exec          string
	Icon          string
	Categories    string
	AppImage      string
	Version       string
	Terminal      bool
	StartupNotify bool
}

func Generate(spec Spec) string {
	cats := spec.Categories
	if cats == "" {
		cats = "Utility;"
	}
	if !strings.HasSuffix(cats, ";") {
		cats += ";"
	}
	out := []string{"[Desktop Entry]", "Type=Application", "Name=" + escape(spec.Name)}
	if spec.Comment != "" {
		out = append(out, "Comment="+escape(spec.Comment))
	}
	out = append(out,
		"Exec="+spec.Exec,
		"Icon="+spec.Icon,
		"Categories="+cats,
		fmt.Sprintf("Terminal=%t", spec.Terminal),
		fmt.Sprintf("StartupNotify=%t", spec.StartupNotify),
	)
	if spec.Version != "" {
		out = append(out, "X-AppImage-Version="+spec.Version)
	}
	out = append(out,
		ManagedKey+"="+ManagedValue,
		ManagedIDKey+"="+spec.ID,
		ManagedAppPathKey+"="+spec.AppImage,
	)
	return strings.Join(out, "\n") + "\n"
}

func escape(s string) string {
	// minimal; desktop files are mostly plain text
	s = strings.ReplaceAll(s, "\n", " ")
	return s
}

func ParseFile(path string) (*Entry, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	e := &Entry{Path: path, Values: map[string]string{}}
	inDesktop := false
	sc := bufio.NewScanner(bytes.NewReader(data))
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
			sec := strings.TrimSuffix(strings.TrimPrefix(line, "["), "]")
			inDesktop = (sec == "Desktop Entry")
			continue
		}
		if !inDesktop {
			continue
		}
		k, v, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		k = strings.TrimSpace(k)
		v = strings.TrimSpace(v)
		if _, exists := e.Values[k]; !exists {
			e.Values[k] = v
		}
	}
	return e, nil
}

func ListManaged(dir string) ([]*Entry, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var out []*Entry
	for _, de := range entries {
		if de.IsDir() || !strings.HasSuffix(de.Name(), ".desktop") {
			continue
		}
		p := filepath.Join(dir, de.Name())
		e, err := ParseFile(p)
		if err != nil {
			continue
		}
		if e.IsManaged() {
			out = append(out, e)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Path < out[j].Path })
	return out, nil
}

func SlugID(name string) string {
	s := strings.ToLower(strings.TrimSpace(name))
	var b strings.Builder
	prevDash := false
	for _, r := range s {
		ok := (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9')
		if ok {
			b.WriteRune(r)
			prevDash = false
		} else if !prevDash {
			b.WriteByte('-')
			prevDash = true
		}
	}
	out := strings.Trim(b.String(), "-")
	if out == "" {
		out = "app"
	}
	return "appman-" + out
}

func DesktopPath(dir, id string) string {
	return filepath.Join(dir, id+".desktop")
}
