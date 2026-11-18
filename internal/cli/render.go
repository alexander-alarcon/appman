package cli

import (
	"fmt"
	"io"
	"text/tabwriter"

	"appman/internal/appimage"
	"appman/internal/logging"
	"appman/internal/manager"
)

func printImages(out io.Writer, images []appimage.Image, log *logging.Logger) error {
	w := tabwriter.NewWriter(out, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "NAME\tFILE\tSIZE")

	for _, img := range images {
		fmt.Fprintf(w, "%s\t%s\t%s\n", img.NameGuess, img.Filename, formatSize(img.Size))

		if log.Level() >= logging.LevelV {
			log.V("%s", img.Path)
		}
		if log.Level() >= logging.LevelVV {
			log.VV("mtime=%s mode=%s", img.ModTime.Format("2006-01-02 15:04:05"), img.Mode.String())
		}
		if log.Level() >= logging.LevelVVV {
			log.Debug("full path: %s", img.Path)
		}
	}

	return w.Flush()
}

func printInspect(out io.Writer, info *appimage.InspectInfo, log *logging.Logger) error {
	w := tabwriter.NewWriter(out, 0, 0, 2, ' ', 0)

	fmt.Fprintf(w, "Name:\t%s", info.Metadata.Name)
	if origin := info.Origins["name"]; origin != "" {
		fmt.Fprintf(w, " (source: %s)", origin)
	}
	fmt.Fprintln(w)

	if info.Metadata.Version != "" {
		fmt.Fprintf(w, "Version:\t%s", info.Metadata.Version)
		if origin := info.Origins["version"]; origin != "" {
			fmt.Fprintf(w, " (source: %s)", origin)
		}
		fmt.Fprintln(w)
	}

	fmt.Fprintf(w, "File:\t%s\n", info.Image.Filename)
	fmt.Fprintf(w, "Path:\t%s\n", info.Image.Path)
	fmt.Fprintf(w, "Type:\t%s\n", info.Type)

	if info.Offset > 0 {
		fmt.Fprintf(w, "Offset:\t%d\n", info.Offset)
	}

	fmt.Fprintf(w, "Size:\t%s\n", formatSize(info.Image.Size))

	fmt.Fprintln(w)
	fmt.Fprintln(w, "Metadata")

	if info.Metadata.Categories != "" {
		fmt.Fprintf(w, "Categories:\t%s\n", info.Metadata.Categories)
	}
	if info.Metadata.Comment != "" {
		fmt.Fprintf(w, "Comment:\t%s\n", info.Metadata.Comment)
	}
	if info.Metadata.Icon != "" {
		fmt.Fprintf(w, "Icon:\t%s\n", info.Metadata.Icon)
	}

	fmt.Fprintln(w)
	fmt.Fprintln(w, "Integration")

	if info.DesktopFile != "" {
		fmt.Fprintln(w, "Desktop entry:\tFound")
	} else {
		fmt.Fprintln(w, "Desktop entry:\tNot found")
	}

	if info.IconFile != "" {
		fmt.Fprintln(w, "Icon file:\tFound")
	} else {
		fmt.Fprintln(w, "Icon file:\tNot found")
	}

	if info.IconExtract != "" {
		fmt.Fprintf(w, "Extracted icon:\t%s\n", info.IconExtract)
	}

	fmt.Fprintln(w)
	if info.Degraded {
		fmt.Fprintln(w, "Status:\tDegraded")
	} else {
		fmt.Fprintln(w, "Status:\tOK")
	}

	if err := w.Flush(); err != nil {
		return err
	}

	for _, warning := range info.Warnings {
		log.Warn("%s", warning)
	}

	if log.Level() >= logging.LevelVVV {
		log.Debug("origins=%v extractTool=%s", info.Origins, info.ExtractTool)
	}

	return nil
}

func printPlan(out io.Writer, plan *manager.Plan, log *logging.Logger) {
	c, u, r, n := plan.Summary()
	fmt.Fprintf(out, "Plan: %d to create, %d to update, %d to remove, %d unchanged.\n", c, u, r, n)

	for _, a := range plan.Actions {
		switch a.Kind {
		case manager.ActionNoop:
			if log.Level() >= logging.LevelV {
				fmt.Fprintf(out, "  [unchanged] %s - %s\n", a.Name, a.Reason)
			}
		case manager.ActionCreate:
			fmt.Fprintf(out, "  [create]     %s -> %s\n", a.Name, a.DesktopPath)
		case manager.ActionUpdate:
			fmt.Fprintf(out, "  [update]     %s -> %s\n", a.Name, a.DesktopPath)
		case manager.ActionRemove:
			fmt.Fprintf(out, "  [remove]     %s (%s)\n", displayActionName(a), a.DesktopPath)
		}

		if log.Level() >= logging.LevelVV && a.IconPath != "" {
			fmt.Fprintf(out, "               Icon: %s\n", a.IconPath)
		}
	}
}

func displayStatus(status manager.Status) string {
	switch status {
	case manager.StatusRegistered:
		return "Installed"
	case manager.StatusUnregistered:
		return "Available"
	case manager.StatusOrphan:
		return "Orphaned"
	default:
		if status == "" {
			return "Unknown"
		}
		return string(status)
	}
}

func displayActionName(a manager.Action) string {
	if a.Name != "" {
		return a.Name
	}
	if a.ID != "" {
		return a.ID
	}
	return a.DesktopPath
}

func formatSize(size int64) string {
	const unit = 1024
	if size < unit {
		return fmt.Sprintf("%d B", size)
	}
	div, exp := int64(unit), 0
	for n := size / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	units := []string{"KiB", "MiB", "GiB", "TiB"}
	if exp >= len(units) {
		exp = len(units) - 1
	}
	return fmt.Sprintf("%.1f %s", float64(size)/float64(div), units[exp])
}
