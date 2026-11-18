package cli

import (
	"fmt"
	"text/tabwriter"

	"appman/internal/manager"

	"github.com/spf13/cobra"
)

func (a *App) newListCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List applications and their integration status",
		RunE:  a.runList,
	}

	cmd.Flags().BoolVar(&a.listInstalled, "installed", false, "Show only integrated applications")
	cmd.Flags().BoolVar(&a.listAvailable, "available", false, "Show only applications without desktop integration")
	cmd.Flags().BoolVar(&a.listOrphaned, "orphaned", false, "Show only orphaned desktop entries")

	return cmd
}

func (a *App) runList(cmd *cobra.Command, args []string) error {
	if a.countListFilters() > 1 {
		return fmt.Errorf("choose only one list filter")
	}

	plan, err := manager.BuildPlan(a.cfg)
	if err != nil {
		return err
	}

	states := a.filterStates(plan.States)
	if len(states) == 0 {
		fmt.Fprintln(cmd.OutOrStdout(), "No applications found.")
		return nil
	}

	w := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "NAME\tSTATUS\tVERSION\tFILE")

	var installed, available, orphaned int
	for _, st := range states {
		name, version, file := "", "", ""
		if st.Inspect != nil {
			name = st.Inspect.DisplayName()
			version = st.Inspect.Metadata.Version
			file = st.Inspect.Image.Filename
		} else if st.Desktop != nil {
			name = st.Desktop.Get("Name")
			file = "(missing AppImage)"
		}

		status := displayStatus(st.Status)
		switch status {
		case "Installed":
			installed++
		case "Available":
			available++
		case "Orphaned":
			orphaned++
		}

		fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", name, status, version, file)

		if a.log.Level() >= 1 && st.Inspect != nil { // LevelV
			a.log.V("%s", st.Inspect.Image.Path)
		}
		if a.log.Level() >= 2 && st.Desktop != nil { // LevelVV
			a.log.VV("desktop=%s", st.Desktop.Path)
		}
	}

	if err := w.Flush(); err != nil {
		return err
	}

	fmt.Fprintf(cmd.OutOrStdout(), "\n%d applications: %d installed, %d available, %d orphaned.\n",
		len(states), installed, available, orphaned)
	return nil
}

func (a *App) countListFilters() int {
	count := 0
	if a.listInstalled {
		count++
	}
	if a.listAvailable {
		count++
	}
	if a.listOrphaned {
		count++
	}
	return count
}

func (a *App) filterStates(states []manager.AppState) []manager.AppState {
	if !a.listInstalled && !a.listAvailable && !a.listOrphaned {
		return states
	}

	var filtered []manager.AppState
	for _, st := range states {
		status := displayStatus(st.Status)
		if a.listInstalled && status == "Installed" {
			filtered = append(filtered, st)
		}
		if a.listAvailable && status == "Available" {
			filtered = append(filtered, st)
		}
		if a.listOrphaned && status == "Orphaned" {
			filtered = append(filtered, st)
		}
	}
	return filtered
}
