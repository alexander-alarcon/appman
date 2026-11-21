package desktop

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestGenerateAndParseRoundTrip(t *testing.T) {
	spec := Spec{
		ID:            "appman-genoffice",
		Name:          "GenOffice",
		Comment:       "Office suite",
		Exec:          "/home/user/Applications/AppImages/GenOffice.AppImage",
		Icon:          "/home/user/.local/share/icons/appman/appman-genoffice.png",
		Categories:    "Office;",
		AppImage:      "/home/user/Applications/AppImages/GenOffice.AppImage",
		Version:       "0.13.18",
		StartupNotify: true,
	}
	content := Generate(spec)

	for _, want := range []string{
		"[Desktop Entry]",
		"Name=GenOffice",
		"Comment=Office suite",
		"Categories=Office;",
		"X-Appman-Managed=true",
		"X-Appman-ID=appman-genoffice",
		"X-Appman-AppImage=/home/user/Applications/AppImages/GenOffice.AppImage",
		"X-AppImage-Version=0.13.18",
	} {
		if !strings.Contains(content, want) {
			t.Fatalf("generated desktop missing %q:\n%s", want, content)
		}
	}

	dir := t.TempDir()
	path := filepath.Join(dir, "appman-genoffice.desktop")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	e, err := ParseFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !e.IsManaged() {
		t.Fatal("expected managed entry")
	}
	if got := e.Get("Name"); got != "GenOffice" {
		t.Fatalf("Name=%q", got)
	}
	if got := e.Get(ManagedIDKey); got != "appman-genoffice" {
		t.Fatalf("ID=%q", got)
	}
}

func TestListManagedFiltersUnmanaged(t *testing.T) {
	dir := t.TempDir()
	managed := Generate(Spec{ID: "appman-a", Name: "A", Exec: "/a.AppImage", Icon: "a", Categories: "Utility;", AppImage: "/a.AppImage"})
	if err := os.WriteFile(filepath.Join(dir, "appman-a.desktop"), []byte(managed), 0o644); err != nil {
		t.Fatal(err)
	}
	unmanaged := "[Desktop Entry]\nType=Application\nName=Other\nExec=/bin/other\n"
	if err := os.WriteFile(filepath.Join(dir, "other.desktop"), []byte(unmanaged), 0o644); err != nil {
		t.Fatal(err)
	}
	got, err := ListManaged(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].Get(ManagedIDKey) != "appman-a" {
		t.Fatalf("unexpected managed entries: %+v", got)
	}
}

func TestSlugID(t *testing.T) {
	tests := []struct{ in, want string }{
		{"GenOffice", "appman-genoffice"},
		{"My App 1.2", "appman-my-app-1-2"},
		{"  ", "appman-app"},
	}
	for _, tt := range tests {
		if got := SlugID(tt.in); got != tt.want {
			t.Fatalf("SlugID(%q)=%q want %q", tt.in, got, tt.want)
		}
	}
}
