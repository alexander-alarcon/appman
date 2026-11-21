package cli

import (
	"testing"

	"appman/internal/appimage"
	"appman/internal/manager"
)

func TestFindImage(t *testing.T) {
	images := []appimage.Image{
		{NameGuess: "GenOffice", Filename: "GenOffice-0.13.18.AppImage", Path: "/apps/GenOffice-0.13.18.AppImage"},
		{NameGuess: "Shotcut", Filename: "shotcut-linux-x86_64.AppImage", Path: "/apps/shotcut-linux-x86_64.AppImage"},
	}
	if _, ok := findImage(images, "genoffice"); !ok {
		t.Fatal("expected to find by name")
	}
	if _, ok := findImage(images, "shotcut-linux-x86_64.AppImage"); !ok {
		t.Fatal("expected to find by filename")
	}
	if _, ok := findImage(images, "missing"); ok {
		t.Fatal("unexpected match")
	}
}

func TestFilterPlanForInstallAndRemove(t *testing.T) {
	plan := &manager.Plan{Actions: []manager.Action{
		{Kind: manager.ActionCreate, ID: "appman-genoffice", Name: "GenOffice", AppPath: "/apps/g.AppImage", DesktopPath: "/d/g.desktop"},
		{Kind: manager.ActionNoop, ID: "appman-shotcut", Name: "Shotcut", AppPath: "/apps/s.AppImage", DesktopPath: "/d/s.desktop"},
		{Kind: manager.ActionRemove, ID: "appman-old", Name: "Old", DesktopPath: "/d/old.desktop"},
	}}

	install := filterPlanFor(plan, "genoffice", false)
	if len(install.Actions) != 1 || install.Actions[0].Kind != manager.ActionCreate {
		t.Fatalf("unexpected install actions: %+v", install.Actions)
	}

	remove := filterPlanFor(plan, "shotcut", true)
	if len(remove.Actions) != 1 || remove.Actions[0].Kind != manager.ActionRemove {
		t.Fatalf("unexpected remove actions: %+v", remove.Actions)
	}

	keepNoRemove := filterPlanFor(plan, "old", false)
	if len(keepNoRemove.Actions) != 0 {
		t.Fatalf("expected no install actions for orphan, got %+v", keepNoRemove.Actions)
	}
}
