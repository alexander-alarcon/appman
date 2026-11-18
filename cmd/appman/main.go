package main

import (
	"fmt"
	"io"
	"os"
	"strings"
	"text/tabwriter"

	"appman/internal/appimage"
	"appman/internal/config"
	"appman/internal/logging"
	"appman/internal/manager"

	"github.com/spf13/cobra"
)

var (
	verbosity int
	appDir    string
	log       *logging.Logger
	cfg       config.Config
	dryRun    bool
	prune     bool

	listInstalled bool
	listAvailable bool
	listOrphaned  bool
)

func main() {
	defaultCfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: loading configuration: %v\n", err)
		os.Exit(1)
	}

	rootCmd := &cobra.Command{
		Use:           "appman",
		Short:         "Manage your AppImages",
		SilenceUsage:  true,
		SilenceErrors: true,

		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			log = logging.New(logging.LevelFromCount(verbosity))

			c, err := config.Load()
			if err != nil {
				return fmt.Errorf("loading configuration: %w", err)
			}

			cfg = c

			if cmd.Flags().Changed("dir") {
				cfg.General.AppImageDir = appDir
			} else {
				appDir = cfg.General.AppImageDir
			}

			return nil
		},

		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Fprintln(cmd.OutOrStdout(),
				"No command specified. Run 'appman --help' to see available commands.")
			return nil
		},
	}

	rootCmd.PersistentFlags().CountVarP(
		&verbosity, "verbose", "v",
		"Increase verbosity (-v, -vv, -vvv)",
	)

	rootCmd.PersistentFlags().StringVar(
		&appDir, "dir", defaultCfg.General.AppImageDir,
		"Directory containing AppImages",
	)

	// Commands

	scanCmd := &cobra.Command{
		Use:   "scan",
		Short: "Discover AppImages in the configured directory",
		RunE:  runScan,
	}

	listCmd := &cobra.Command{
		Use:   "list",
		Short: "List applications and their integration status",
		RunE:  runList,
	}

	listCmd.Flags().BoolVar(
		&listInstalled, "installed", false,
		"Show only integrated applications",
	)

	listCmd.Flags().BoolVar(
		&listAvailable, "available", false,
		"Show only applications without desktop integration",
	)

	listCmd.Flags().BoolVar(
		&listOrphaned, "orphaned", false,
		"Show only orphaned desktop entries",
	)

	infoCmd := &cobra.Command{
		Use:   "info <app>",
		Short: "Show detailed information about an AppImage",
		Args:  cobra.ExactArgs(1),
		RunE:  runInfo,
	}

	syncCmd := &cobra.Command{
		Use:   "sync",
		Short: "Synchronize AppImage integrations",
		RunE:  runSync,
	}

	syncCmd.Flags().BoolVar(
		&dryRun, "dry-run", false,
		"Show planned changes without applying them",
	)

	syncCmd.Flags().BoolVar(
		&prune, "prune", false,
		"Remove orphaned managed desktop entries",
	)

	installCmd := &cobra.Command{
		Use:   "install <app>",
		Short: "Integrate an AppImage with the desktop",
		Args:  cobra.ExactArgs(1),
		RunE:  runInstall,
	}

	installCmd.Flags().BoolVar(
		&dryRun, "dry-run", false,
		"Show planned changes without applying them",
	)

	removeCmd := &cobra.Command{
		Use:   "remove <app>",
		Short: "Remove an AppImage desktop integration",
		Args:  cobra.ExactArgs(1),
		RunE:  runRemove,
	}

	removeCmd.Flags().BoolVar(
		&dryRun, "dry-run", false,
		"Show planned changes without applying them",
	)

	rootCmd.AddCommand(
		scanCmd,
		listCmd,
		infoCmd,
		syncCmd,
		installCmd,
		removeCmd,
	)

	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

// scan discovers AppImages in the configured directory.

func runScan(cmd *cobra.Command, args []string) error {
	log.Info("Scanning for AppImages in %s...", appDir)

	res, err := appimage.Scan(appDir)
	if err != nil {
		return err
	}

	for _, scanErr := range res.Errors {
		log.Warn("%v", scanErr)
	}

	if len(res.Images) == 0 {
		fmt.Fprintln(cmd.OutOrStdout(), "No AppImages found.")
		return nil
	}

	fmt.Fprintf(
		cmd.OutOrStdout(),
		"Found %d AppImages.\n\n",
		len(res.Images),
	)

	return printImages(cmd.OutOrStdout(), res.Images)
}

// list displays the application inventory and integration status.

func runList(cmd *cobra.Command, args []string) error {
	if countListFilters() > 1 {
		return fmt.Errorf("choose only one list filter")
	}

	plan, err := manager.BuildPlan(cfg)
	if err != nil {
		return err
	}

	states := filterStates(plan.States)

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

		fmt.Fprintf(
			w,
			"%s\t%s\t%s\t%s\n",
			name,
			status,
			version,
			file,
		)

		if log.Level() >= logging.LevelV && st.Inspect != nil {
			log.V("%s", st.Inspect.Image.Path)
		}

		if log.Level() >= logging.LevelVV && st.Desktop != nil {
			log.VV("desktop=%s", st.Desktop.Path)
		}
	}

	if err := w.Flush(); err != nil {
		return err
	}

	total := len(states)

	fmt.Fprintf(
		cmd.OutOrStdout(),
		"\n%d applications: %d installed, %d available, %d orphaned.\n",
		total,
		installed,
		available,
		orphaned,
	)

	return nil
}

// countListFilters returns the number of active list filters.

func countListFilters() int {
	count := 0

	if listInstalled {
		count++
	}
	if listAvailable {
		count++
	}
	if listOrphaned {
		count++
	}

	return count
}

// filterStates selects application states according to the requested filter.

func filterStates(states []manager.AppState) []manager.AppState {
	if !listInstalled && !listAvailable && !listOrphaned {
		return states
	}

	var filtered []manager.AppState

	for _, st := range states {
		status := displayStatus(st.Status)

		if listInstalled && status == "Installed" {
			filtered = append(filtered, st)
		}

		if listAvailable && status == "Available" {
			filtered = append(filtered, st)
		}

		if listOrphaned && status == "Orphaned" {
			filtered = append(filtered, st)
		}
	}

	return filtered
}

// displayStatus translates internal manager states into user-facing labels.

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

// printImages displays discovered AppImages.

func printImages(out io.Writer, images []appimage.Image) error {
	w := tabwriter.NewWriter(out, 0, 0, 2, ' ', 0)

	fmt.Fprintln(w, "NAME\tFILE\tSIZE")

	for _, img := range images {
		fmt.Fprintf(
			w,
			"%s\t%s\t%s\n",
			img.NameGuess,
			img.Filename,
			formatSize(img.Size),
		)

		if log.Level() >= logging.LevelV {
			log.V("%s", img.Path)
		}

		if log.Level() >= logging.LevelVV {
			log.VV(
				"mtime=%s mode=%s",
				img.ModTime.Format("2006-01-02 15:04:05"),
				img.Mode.String(),
			)
		}

		if log.Level() >= logging.LevelVVV {
			log.Debug("full path: %s", img.Path)
		}
	}

	return w.Flush()
}

// info displays detailed information about an AppImage.

func runInfo(cmd *cobra.Command, args []string) error {
	query := args[0]

	res, err := appimage.Scan(appDir)
	if err != nil {
		return err
	}

	img, ok := findImage(res.Images, query)
	if !ok {
		return fmt.Errorf("AppImage not found: %s", query)
	}

	info, err := appimage.Inspect(img.Path)
	if err != nil {
		return err
	}

	return printInspect(cmd.OutOrStdout(), info)
}

// findImage searches for an AppImage by filename, path or partial name.

func findImage(images []appimage.Image, query string) (appimage.Image, bool) {
	q := strings.ToLower(query)

	for _, img := range images {
		if strings.EqualFold(img.Filename, query) ||
			img.Path == query {
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

// printInspect displays AppImage metadata and integration information.

func printInspect(out io.Writer, info *appimage.InspectInfo) error {
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
		log.Debug(
			"origins=%v extractTool=%s",
			info.Origins,
			info.ExtractTool,
		)
	}

	return nil
}

// sync reconciles desktop integrations with discovered AppImages.

func runSync(cmd *cobra.Command, args []string) error {
	plan, err := manager.BuildPlan(cfg)
	if err != nil {
		return err
	}

	printPlan(cmd.OutOrStdout(), plan)

	if dryRun {
		fmt.Fprintln(cmd.OutOrStdout(), "\nDry run: no changes were made.")
		return nil
	}

	if err := manager.Apply(plan, prune); err != nil {
		return err
	}

	if hasOrphans(plan) && !prune {
		log.Warn("Orphaned desktop entries detected.")
		log.Warn("Run 'appman sync --prune' to remove them.")
	}

	log.OK("Synchronization complete.")
	return nil
}

// install creates or updates an AppImage desktop integration.

func runInstall(cmd *cobra.Command, args []string) error {
	query := args[0]

	plan, err := manager.BuildPlan(cfg)
	if err != nil {
		return err
	}

	filtered := filterPlanFor(plan, query, false)

	if len(filtered.Actions) == 0 {
		return fmt.Errorf("no pending integration changes for %q", query)
	}

	printPlan(cmd.OutOrStdout(), filtered)

	if dryRun {
		fmt.Fprintln(cmd.OutOrStdout(), "\nDry run: no changes were made.")
		return nil
	}

	if err := manager.Apply(filtered, false); err != nil {
		return err
	}

	log.OK("Integration complete for %s.", query)
	return nil
}

// remove deletes a managed desktop integration.

func runRemove(cmd *cobra.Command, args []string) error {
	query := args[0]

	plan, err := manager.BuildPlan(cfg)
	if err != nil {
		return err
	}

	filtered := filterPlanFor(plan, query, true)

	if len(filtered.Actions) == 0 {
		return fmt.Errorf("no managed integration found for %q", query)
	}

	printPlan(cmd.OutOrStdout(), filtered)

	if dryRun {
		fmt.Fprintln(cmd.OutOrStdout(), "\nDry run: no changes were made.")
		return nil
	}

	if err := manager.Apply(filtered, true); err != nil {
		return err
	}

	log.OK("Removed integration for %s.", query)
	return nil
}

// filterPlanFor selects actions matching an application query.

func filterPlanFor(
	plan *manager.Plan,
	query string,
	removeOnly bool,
) *manager.Plan {
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

// printPlan displays the operations planned by the manager.

func printPlan(out io.Writer, plan *manager.Plan) {
	c, u, r, n := plan.Summary()

	fmt.Fprintf(
		out,
		"Plan: %d to create, %d to update, %d to remove, %d unchanged.\n",
		c, u, r, n,
	)

	for _, a := range plan.Actions {
		switch a.Kind {
		case manager.ActionNoop:
			if log.Level() >= logging.LevelV {
				fmt.Fprintf(out, "  [unchanged] %s - %s\n", a.Name, a.Reason)
			}

		case manager.ActionCreate:
			fmt.Fprintf(out, "  [create]     %s -> %s\n",
				a.Name, a.DesktopPath)

		case manager.ActionUpdate:
			fmt.Fprintf(out, "  [update]     %s -> %s\n",
				a.Name, a.DesktopPath)

		case manager.ActionRemove:
			fmt.Fprintf(out, "  [remove]     %s (%s)\n",
				displayActionName(a), a.DesktopPath)
		}

		if log.Level() >= logging.LevelVV && a.IconPath != "" {
			fmt.Fprintf(out, "               Icon: %s\n", a.IconPath)
		}
	}
}

// displayActionName returns the most useful identifier for an action.

func displayActionName(a manager.Action) string {
	if a.Name != "" {
		return a.Name
	}

	if a.ID != "" {
		return a.ID
	}

	return a.DesktopPath
}

// hasOrphans checks whether the plan contains removal operations.

func hasOrphans(plan *manager.Plan) bool {
	for _, a := range plan.Actions {
		if a.Kind == manager.ActionRemove {
			return true
		}
	}

	return false
}

// formatSize converts bytes into human-readable IEC units.

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

	return fmt.Sprintf(
		"%.1f %s",
		float64(size)/float64(div),
		units[exp],
	)
}
