package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"text/tabwriter"

	"appman/internal/appimage"
	"appman/internal/logging"

	"github.com/spf13/cobra"
)

var (
	verbosity int
	appDir    string
	log       *logging.Logger
)

func main() {
	rootCmd := &cobra.Command{
		Use:           "appman",
		Short:         "Manage your AppImages",
		SilenceUsage:  true,
		SilenceErrors: true,

		PersistentPreRun: func(cmd *cobra.Command, args []string) {
			log = logging.New(logging.LevelFromCount(verbosity))
		},

		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Fprintln(cmd.OutOrStdout(),
				"Interactive mode is not available yet. Use 'appman --help' to see available commands.")
			return nil
		},
	}

	rootCmd.PersistentFlags().CountVarP(
		&verbosity, "verbose", "v",
		"Increase verbosity (-v, -vv, -vvv)",
	)

	rootCmd.PersistentFlags().StringVar(
		&appDir, "dir", defaultAppDir(),
		"Directory containing AppImages",
	)

	scanCmd := &cobra.Command{
		Use:   "scan",
		Short: "Scan for AppImages",
		RunE:  runScan,
	}

	listCmd := &cobra.Command{
		Use:   "list",
		Short: "List detected AppImages",
		RunE:  runList,
	}

	infoCmd := &cobra.Command{
		Use:   "info <app>",
		Short: "Show detailed information about an AppImage",
		Args:  cobra.ExactArgs(1),
		RunE:  runInfo,
	}

	rootCmd.AddCommand(scanCmd, listCmd, infoCmd)

	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

func defaultAppDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return "."
	}

	return filepath.Join(home, "Applications", "AppImages")
}

func runScan(cmd *cobra.Command, args []string) error {
	log.Info("Scanning %s", appDir)

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

	fmt.Fprintf(cmd.OutOrStdout(), "Found %d AppImages.\n\n", len(res.Images))

	return printImages(cmd.OutOrStdout(), res.Images, log)
}

func runList(cmd *cobra.Command, args []string) error {
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

	if err := printImages(cmd.OutOrStdout(), res.Images, log); err != nil {
		return err
	}

	fmt.Fprintf(cmd.OutOrStdout(), "\n%d AppImages found.\n", len(res.Images))
	return nil
}

func printImages(out io.Writer, images []appimage.Image, logger *logging.Logger) error {
	w := tabwriter.NewWriter(out, 0, 0, 2, ' ', 0)

	fmt.Fprintln(w, "NAME\tFILE\tSIZE")

	for _, img := range images {
		fmt.Fprintf(w, "%s\t%s\t%s\n",
			img.NameGuess,
			img.Filename,
			formatSize(img.Size),
		)

		if logger.Level() >= logging.LevelV {
			logger.V("%s", img.Path)
		}

		if logger.Level() >= logging.LevelVV {
			logger.VV("mtime=%s mode=%s",
				img.ModTime.Format("2006-01-02 15:04:05"),
				img.Mode.String(),
			)
		}

		if logger.Level() >= logging.LevelVVV {
			logger.Debug("full path: %s", img.Path)
		}
	}

	return w.Flush()
}

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

	return printInspect(cmd.OutOrStdout(), info, log)
}

func findImage(images []appimage.Image, query string) (appimage.Image, bool) {
	q := strings.ToLower(query)

	// Exact filename or path.
	for _, img := range images {
		if strings.EqualFold(img.Filename, query) || img.Path == query {
			return img, true
		}
	}

	// Partial name or filename match.
	for _, img := range images {
		if strings.Contains(strings.ToLower(img.NameGuess), q) ||
			strings.Contains(strings.ToLower(img.Filename), q) {
			return img, true
		}
	}

	return appimage.Image{}, false
}

func printInspect(out io.Writer, info *appimage.InspectInfo, logger *logging.Logger) error {
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
		fmt.Fprintf(w, "Desktop entry:\t%s\n", info.DesktopFile)
	} else {
		fmt.Fprintln(w, "Desktop entry:\tNot found")
	}

	if info.IconFile != "" {
		fmt.Fprintf(w, "Icon file:\t%s\n", info.IconFile)
	} else {
		fmt.Fprintln(w, "Icon file:\tNot found")
	}

	if info.IconExtract != "" {
		fmt.Fprintf(w, "Extracted icon:\t%s\n", info.IconExtract)
	}

	if info.Degraded {
		fmt.Fprintln(w)
		fmt.Fprintln(w, "Status:\tDegraded")
	} else {
		fmt.Fprintln(w)
		fmt.Fprintln(w, "Status:\tOK")
	}

	if err := w.Flush(); err != nil {
		return err
	}

	for _, warning := range info.Warnings {
		logger.Warn("%s", warning)
	}

	if logger.Level() >= logging.LevelVVV {
		logger.Debug("origins=%v extractTool=%s",
			info.Origins,
			info.ExtractTool,
		)
	}

	return nil
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

	return fmt.Sprintf("%.1f %s",
		float64(size)/float64(div),
		units[exp],
	)
}
