package manager

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"appman/internal/config"
	"appman/internal/desktop"
)

func testConfig(t *testing.T) config.Config {
	t.Helper()
	base := t.TempDir()
	return config.Config{
		General: config.GeneralConfig{
			AppImageDir: filepath.Join(base, "apps"),
			DesktopDir:  filepath.Join(base, "applications"),
			IconDir:     filepath.Join(base, "icons"),
		},
		Scanner: config.ScannerConfig{ExtractIcons: true},
		Desktop: config.DesktopConfig{Categories: []string{"Utility"}, StartupNotify: true},
	}
}

func writeFakeAppImage(t *testing.T, dir, name string) string {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	p := filepath.Join(dir, name)
	if err := os.WriteFile(p, []byte("not-a-real-appimage"), 0o755); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestBuildPlanCreateAndIdempotent(t *testing.T) {
	cfg := testConfig(t)
	writeFakeAppImage(t, cfg.General.AppImageDir, "MyApp-1.0.AppImage")

	plan, err := BuildPlan(cfg)
	if err != nil {
		t.Fatal(err)
	}
	create, update, remove, noop := plan.Summary()
	if create != 1 || update != 0 || remove != 0 || noop != 0 {
		t.Fatalf("unexpected summary: create=%d update=%d remove=%d noop=%d", create, update, remove, noop)
	}

	if err := Apply(plan, false); err != nil {
		t.Fatal(err)
	}

	// second run should be noop
	plan2, err := BuildPlan(cfg)
	if err != nil {
		t.Fatal(err)
	}
	create, update, remove, noop = plan2.Summary()
	if create != 0 || update != 0 || remove != 0 || noop != 1 {
		t.Fatalf("expected idempotent noop, got create=%d update=%d remove=%d noop=%d", create, update, remove, noop)
	}
}

func TestBuildPlanOrphanAndPrune(t *testing.T) {
	cfg := testConfig(t)
	appPath := writeFakeAppImage(t, cfg.General.AppImageDir, "Old.AppImage")

	plan, err := BuildPlan(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if err := Apply(plan, false); err != nil {
		t.Fatal(err)
	}

	// remove the appimage, plan should mark orphan
	if err := os.Remove(appPath); err != nil {
		t.Fatal(err)
	}
	plan2, err := BuildPlan(cfg)
	if err != nil {
		t.Fatal(err)
	}
	create, update, remove, noop := plan2.Summary()
	if create != 0 || update != 0 || remove != 1 || noop != 0 {
		t.Fatalf("unexpected orphan summary: create=%d update=%d remove=%d noop=%d", create, update, remove, noop)
	}

	// apply without prune should keep file
	if err := Apply(plan2, false); err != nil {
		t.Fatal(err)
	}
	entries, err := desktop.ListManaged(cfg.General.DesktopDir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected orphan desktop kept, got %d", len(entries))
	}

	// apply with prune should remove
	if err := Apply(plan2, true); err != nil {
		t.Fatal(err)
	}
	entries, err = desktop.ListManaged(cfg.General.DesktopDir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Fatalf("expected orphan desktop removed, got %d", len(entries))
	}
}

func TestBuildPlanDoesNotTouchUnmanaged(t *testing.T) {
	cfg := testConfig(t)
	if err := os.MkdirAll(cfg.General.AppImageDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(cfg.General.DesktopDir, 0o755); err != nil {
		t.Fatal(err)
	}
	unmanaged := filepath.Join(cfg.General.DesktopDir, "firefox.desktop")
	content := "[Desktop Entry]\nType=Application\nName=Firefox\nExec=firefox\n"
	if err := os.WriteFile(unmanaged, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	plan, err := BuildPlan(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if err := Apply(plan, true); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(unmanaged)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != content {
		t.Fatalf("unmanaged desktop was modified:\n%s", got)
	}
}

func TestBuildPlanUpdatesStaleEntry(t *testing.T) {
	cfg := testConfig(t)
	writeFakeAppImage(t, cfg.General.AppImageDir, "MyApp.AppImage")

	plan, err := BuildPlan(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if err := Apply(plan, false); err != nil {
		t.Fatal(err)
	}

	// tamper managed desktop
	entries, err := desktop.ListManaged(cfg.General.DesktopDir)
	if err != nil || len(entries) != 1 {
		t.Fatalf("expected 1 managed entry, got %d err=%v", len(entries), err)
	}
	if err := os.WriteFile(entries[0].Path, []byte("[Desktop Entry]\nName=Broken\nX-Appman-Managed=true\nX-Appman-ID=appman-myapp\nX-Appman-AppImage=x\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	plan2, err := BuildPlan(cfg)
	if err != nil {
		t.Fatal(err)
	}
	create, update, remove, noop := plan2.Summary()
	if update != 1 || create != 0 || remove != 0 || noop != 0 {
		t.Fatalf("expected update, got create=%d update=%d remove=%d noop=%d", create, update, remove, noop)
	}
	if err := Apply(plan2, false); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(entries[0].Path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(got), "Name=MyApp") {
		t.Fatalf("expected repaired desktop, got:\n%s", got)
	}
}
