package cli

import (
	"strings"

	"appman/internal/appimage"
	"appman/internal/manager"
)

func findImage(images []appimage.Image, query string) (appimage.Image, bool) {
	q := strings.ToLower(query)

	for _, img := range images {
		if strings.EqualFold(img.Filename, query) || img.Path == query {
			return img, true
		}
	}

	for _, img := range images {
		if strings.Contains(strings.ToLower(img.NameGuess), q) ||
			strings.Contains(strings.ToLower(img.Filename), q) {
			return img, true
		}
	}

	return appimage.Image{}, false
}

func filterPlanFor(plan *manager.Plan, query string, removeOnly bool) *manager.Plan {
	q := strings.ToLower(query)
	out := &manager.Plan{}

	for _, a := range plan.Actions {
		match := strings.Contains(strings.ToLower(a.Name), q) ||
			strings.Contains(strings.ToLower(a.AppPath), q) ||
			strings.Contains(strings.ToLower(a.ID), q)
		if !match {
			continue
		}

		if removeOnly {
			if a.Kind == manager.ActionRemove {
				out.Actions = append(out.Actions, a)
			} else if a.DesktopPath != "" {
				out.Actions = append(out.Actions, manager.Action{
					Kind:        manager.ActionRemove,
					ID:          a.ID,
					Name:        a.Name,
					AppPath:     a.AppPath,
					DesktopPath: a.DesktopPath,
					Reason:      "requested removal",
				})
			}
		} else {
			if a.Kind == manager.ActionRemove {
				continue
			}
			out.Actions = append(out.Actions, a)
		}
	}

	return out
}

func hasOrphans(plan *manager.Plan) bool {
	for _, a := range plan.Actions {
		if a.Kind == manager.ActionRemove {
			return true
		}
	}
	return false
}
