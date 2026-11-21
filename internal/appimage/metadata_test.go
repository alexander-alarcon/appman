package appimage

import "testing"

func TestGuessName(t *testing.T) {
	tests := []struct {
		in, want string
	}{
		{"GenOffice-0.13.18-x86_64.AppImage", "GenOffice-0.13.18-x86_64"},
		{"shotcut-linux-x86_64.AppImage", "shotcut-linux-x86_64"},
		{"MyApp-1.2.3.AppImage", "MyApp"},
		{"simple.appimage", "simple"},
	}
	for _, tt := range tests {
		if got := guessName(tt.in); got != tt.want {
			t.Fatalf("guessName(%q)=%q want %q", tt.in, got, tt.want)
		}
	}
}

func TestParseDesktopEntry(t *testing.T) {
	data := []byte(`[Desktop Entry]
Name=MyApp
Comment=Hello
Icon=myapp
Categories=Utility;Office;
X-AppImage-Version=1.2.3
`)
	md, origins := parseDesktopEntry(data)
	if md.Name != "MyApp" || md.Comment != "Hello" || md.Icon != "myapp" {
		t.Fatalf("unexpected metadata: %+v", md)
	}
	if md.Categories != "Utility;Office;" {
		t.Fatalf("categories=%q", md.Categories)
	}
	if md.Version != "1.2.3" {
		t.Fatalf("version=%q", md.Version)
	}
	if origins["name"] != "desktop" || origins["icon"] != "desktop" {
		t.Fatalf("unexpected origins: %+v", origins)
	}
}

func TestPickDesktopAndIcon(t *testing.T) {
	files := []string{
		"usr/share/icons/hicolor/256x256/apps/myapp.png",
		"usr/share/applications/myapp.desktop",
		"myapp.desktop",
		".DirIcon",
	}
	if got := pickDesktop(files); got != "myapp.desktop" {
		t.Fatalf("pickDesktop=%q", got)
	}
	if got := pickIcon(files, "myapp"); got != "usr/share/icons/hicolor/256x256/apps/myapp.png" {
		t.Fatalf("pickIcon=%q", got)
	}
	if got := pickIcon(files, ""); got != ".DirIcon" {
		t.Fatalf("pickIcon fallback=%q", got)
	}
}
