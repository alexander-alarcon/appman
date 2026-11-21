package manager

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"appman/internal/appimage"
	"appman/internal/config"
	"appman/internal/desktop"
)

type Status string

const (
	StatusRegistered   Status = "registered"
	StatusUnregistered Status = "unregistered"
	StatusOrphan       Status = "orphan"
	StatusConflict     Status = "conflict"
	StatusBroken       Status = "broken"
)

type AppState struct {
	ID          string
	Inspect     *appimage.InspectInfo
	Desktop     *desktop.Entry
	Status      Status
	DesiredPath string
	DesiredIcon string
	Reason      string
}

type ActionKind string

const (
	ActionCreate ActionKind = "create"
	ActionUpdate ActionKind = "update"
	ActionRemove ActionKind = "remove"
	ActionNoop   ActionKind = "noop"
)

type Action struct {
	Kind        ActionKind
	ID          string
	Name        string
	AppPath     string
	DesktopPath string
	IconPath    string
	IconSource  string
	Reason      string
	Content     string
}

type Plan struct {
	Actions []Action
	States  []AppState
}

func BuildPlan(cfg config.Config) (*Plan, error) {
	if err := os.MkdirAll(cfg.General.DesktopDir, 0o755); err != nil {
		return nil, err
	}
	if cfg.Scanner.ExtractIcons {
		if err := os.MkdirAll(cfg.General.IconDir, 0o755); err != nil {
			return nil, err
		}
	}

	scan, err := appimage.Scan(cfg.General.AppImageDir)
	if err != nil {
		return nil, err
	}

	managed, err := desktop.ListManaged(cfg.General.DesktopDir)
	if err != nil {
		return nil, err
	}

	byPath := map[string]*desktop.Entry{}
	byID := map[string]*desktop.Entry{}
	for _, e := range managed {
		p := e.Get(desktop.ManagedAppPathKey)
		if p != "" {
			byPath[p] = e
		}
		id := e.Get(desktop.ManagedIDKey)
		if id != "" {
			byID[id] = e
		}
	}

	plan := &Plan{}
	usedDesktop := map[string]bool{}

	for _, img := range scan.Images {
		info, err := appimage.Inspect(img.Path)
		if err != nil {
			plan.States = append(plan.States, AppState{Inspect: &appimage.InspectInfo{Image: img}, Status: StatusBroken, Reason: err.Error()})
			continue
		}
		id := desktop.SlugID(info.DisplayName())
		state := AppState{ID: id, Inspect: info}
		desiredDesktop := desktop.DesktopPath(cfg.General.DesktopDir, id)

		var matched *desktop.Entry
		if e, ok := byPath[img.Path]; ok {
			matched = e
		} else if e, ok := byID[id]; ok {
			matched = e
		}
		if matched != nil {
			usedDesktop[matched.Path] = true
			state.Desktop = matched
		}

		iconValue, iconSrc, iconDest := ensureIcon(info, cfg)
		state.DesiredIcon = iconValue
		execPath := quoteExec(img.Path)
		cats := info.Metadata.Categories
		if cats == "" && len(cfg.Desktop.Categories) > 0 {
			cats = strings.Join(cfg.Desktop.Categories, ";")
		}
		content := desktop.Generate(desktop.Spec{
			ID:            id,
			Name:          info.DisplayName(),
			Comment:       info.Metadata.Comment,
			Exec:          execPath,
			Icon:          iconValue,
			Categories:    cats,
			AppImage:      img.Path,
			Version:       info.Metadata.Version,
			Terminal:      false,
			StartupNotify: cfg.Desktop.StartupNotify,
		})

		if matched == nil {
			state.Status = StatusUnregistered
			state.DesiredPath = desiredDesktop
			plan.Actions = append(plan.Actions, Action{Kind: ActionCreate, ID: id, Name: info.DisplayName(), AppPath: img.Path, DesktopPath: desiredDesktop, IconPath: iconDest, IconSource: iconSrc, Reason: "missing desktop entry", Content: content})
		} else {
			state.DesiredPath = matched.Path
			existing, _ := os.ReadFile(matched.Path)
			if string(existing) != content || matched.Path != desiredDesktop {
				state.Status = StatusRegistered
				plan.Actions = append(plan.Actions, Action{Kind: ActionUpdate, ID: id, Name: info.DisplayName(), AppPath: img.Path, DesktopPath: desiredDesktop, IconPath: iconDest, IconSource: iconSrc, Reason: "out of date", Content: content})
			} else {
				state.Status = StatusRegistered
				plan.Actions = append(plan.Actions, Action{Kind: ActionNoop, ID: id, Name: info.DisplayName(), AppPath: img.Path, DesktopPath: matched.Path, Reason: "up to date"})
			}
		}
		plan.States = append(plan.States, state)
	}

	// orphans: managed desktop entries whose appimage is missing
	for _, e := range managed {
		if usedDesktop[e.Path] {
			continue
		}
		appPath := e.Get(desktop.ManagedAppPathKey)
		missing := true
		if appPath != "" {
			if _, err := os.Stat(appPath); err == nil {
				missing = false
			}
		}
		if !missing {
			continue
		}
		st := AppState{ID: e.Get(desktop.ManagedIDKey), Desktop: e, Status: StatusOrphan, Reason: "appimage missing"}
		plan.States = append(plan.States, st)
		plan.Actions = append(plan.Actions, Action{Kind: ActionRemove, ID: e.Get(desktop.ManagedIDKey), Name: e.Get("Name"), AppPath: appPath, DesktopPath: e.Path, Reason: "orphan (appimage missing)"})
	}

	sort.Slice(plan.States, func(i, j int) bool {
		ni, nj := "", ""
		if plan.States[i].Inspect != nil {
			ni = plan.States[i].Inspect.DisplayName()
		} else if plan.States[i].Desktop != nil {
			ni = plan.States[i].Desktop.Get("Name")
		}
		if plan.States[j].Inspect != nil {
			nj = plan.States[j].Inspect.DisplayName()
		} else if plan.States[j].Desktop != nil {
			nj = plan.States[j].Desktop.Get("Name")
		}
		return strings.ToLower(ni) < strings.ToLower(nj)
	})
	return plan, nil
}

func ensureIcon(info *appimage.InspectInfo, cfg config.Config) (iconValue string, iconSource string, iconDest string) {
	// if we have extracted icon, plan to copy to managed icon dir
	if info.IconExtract != "" && cfg.Scanner.ExtractIcons {
		ext := filepath.Ext(info.IconExtract)
		if ext == "" {
			ext = filepath.Ext(info.IconFile)
		}
		if ext == "" {
			ext = ".png"
		}
		id := desktop.SlugID(info.DisplayName())
		dest := filepath.Join(cfg.General.IconDir, id+ext)
		return dest, info.IconExtract, dest
	}
	if info.Metadata.Icon != "" {
		return info.Metadata.Icon, "", ""
	}
	return "application-x-executable", "", ""
}

func copyFile(src, dst string) error {
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer func() { _ = in.Close() }()
	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	if _, err := out.ReadFrom(in); err != nil {
		_ = out.Close()
		return err
	}
	return out.Close()
}

func quoteExec(path string) string {
	// simple quoting for Exec
	if strings.ContainsAny(path, " \t\"'`$\\") {
		return fmt.Sprintf("\"%s\"", strings.ReplaceAll(path, "\"", "\\\""))
	}
	return path
}

func Apply(plan *Plan, applyRemoves bool) error {
	for _, a := range plan.Actions {
		switch a.Kind {
		case ActionCreate, ActionUpdate:
			if a.IconSource != "" && a.IconPath != "" {
				if err := copyFile(a.IconSource, a.IconPath); err != nil {
					return fmt.Errorf("copy icon for %s: %w", a.Name, err)
				}
			}
			if err := os.MkdirAll(filepath.Dir(a.DesktopPath), 0o755); err != nil {
				return err
			}
			if err := os.WriteFile(a.DesktopPath, []byte(a.Content), 0o644); err != nil {
				return err
			}
		case ActionRemove:
			if applyRemoves {
				if err := os.Remove(a.DesktopPath); err != nil && !os.IsNotExist(err) {
					return err
				}
			}
		case ActionNoop:
		}
	}
	return nil
}

func (p *Plan) Summary() (create, update, remove, noop int) {
	for _, a := range p.Actions {
		switch a.Kind {
		case ActionCreate:
			create++
		case ActionUpdate:
			update++
		case ActionRemove:
			remove++
		case ActionNoop:
			noop++
		}
	}
	return
}
