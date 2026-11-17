package main

import (
	"fmt"
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
		Use:   "appman",
		Short: "Manage your AppImages",
		PersistentPreRun: func(cmd *cobra.Command, args []string) {
			log = logging.New(logging.LevelFromCount(verbosity))
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Fprintln(cmd.OutOrStdout(), "appman: TUI not implemented yet. Use `appman scan` or `appman list`.")
			return nil
		},
	}

	rootCmd.PersistentFlags().CountVarP(&verbosity, "verbose", "v", "Increase verbosity (-v, -vv, -vvv)")
	rootCmd.PersistentFlags().StringVar(&appDir, "dir", defaultAppDir(), "Directory containing AppImages")

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
		os.Exit(1)
	}
}

func defaultAppDir() string {
	h, err := os.UserHomeDir()
	if err != nil {
		return "."
	}
	return filepath.Join(h, "Applications", "AppImages")
}

func runScan(cmd *cobra.Command, args []string) error {
	log.Info("Scanning AppImages in %s", appDir)
	res, err := appimage.Scan(appDir)
	if err != nil {
		log.Error("%v", err)
		return err
	}
	for _, e := range res.Errors {
		log.Warn("%v", e)
	}
	log.OK("Found %d AppImage(s)", len(res.Images))
	printImages(res.Images, log)
	return nil
}

func runList(cmd *cobra.Command, args []string) error {
	res, err := appimage.Scan(appDir)
	if err != nil {
		log.Error("%v", err)
		return err
	}
	if len(res.Images) == 0 {
		log.Warn("No AppImages found in %s", appDir)
		return nil
	}
	printImages(res.Images, log)
	return nil
}

func printImages(images []appimage.Image, log *logging.Logger) {
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "NAME\tFILE\tSIZE")
	for _, img := range images {
		fmt.Fprintf(w, "%s\t%s\t%d bytes\n", img.NameGuess, img.Filename, img.Size)
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
	w.Flush()
}

func runInfo(cmd *cobra.Command, args []string) error {
	query := args[0]
	res, err := appimage.Scan(appDir)
	if err != nil {
		log.Error("%v", err)
		return err
	}
	img, ok := findImage(res.Images, query)
	if !ok {
		log.Error("AppImage not found: %s", query)
		return fmt.Errorf("not found: %s", query)
	}
	log.Info("Inspecting %s", img.Path)
	info, err := appimage.Inspect(img.Path)
	if err != nil {
		log.Error("%v", err)
		return err
	}
	printInspect(info)
	return nil
}

func findImage(images []appimage.Image, query string) (appimage.Image, bool) {
	q := strings.ToLower(query)
	// exact filename or path
	for _, img := range images {
		if strings.EqualFold(img.Filename, query) || img.Path == query {
			return img, true
		}
	}
	// contains match on name/filename
	for _, img := range images {
		if strings.Contains(strings.ToLower(img.NameGuess), q) || strings.Contains(strings.ToLower(img.Filename), q) {
			return img, true
		}
	}
	return appimage.Image{}, false
}

func printInspect(info *appimage.InspectInfo) {
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintf(w, "Name:\t%s", info.Metadata.Name)
	if o := info.Origins["name"]; o != "" {
		fmt.Fprintf(w, " (source: %s)", o)
	}
	fmt.Fprintln(w)
	if info.Metadata.Version != "" {
		fmt.Fprintf(w, "Version:\t%s", info.Metadata.Version)
		if o := info.Origins["version"]; o != "" {
			fmt.Fprintf(w, " (source: %s)", o)
		}
		fmt.Fprintln(w)
	}
	fmt.Fprintf(w, "File:\t%s\n", info.Image.Filename)
	fmt.Fprintf(w, "Path:\t%s\n", info.Image.Path)
	fmt.Fprintf(w, "Type:\t%s\n", info.Type)
	if info.Offset > 0 {
		fmt.Fprintf(w, "Offset:\t%d\n", info.Offset)
	}
	fmt.Fprintf(w, "Size:\t%d bytes\n", info.Image.Size)
	if info.Metadata.Categories != "" {
		fmt.Fprintf(w, "Categories:\t%s\n", info.Metadata.Categories)
	}
	if info.Metadata.Comment != "" {
		fmt.Fprintf(w, "Comment:\t%s\n", info.Metadata.Comment)
	}
	if info.Metadata.Icon != "" {
		fmt.Fprintf(w, "Icon:\t%s\n", info.Metadata.Icon)
	}
	if info.DesktopFile != "" {
		fmt.Fprintf(w, "Desktop entry:\t%s\n", info.DesktopFile)
	} else {
		fmt.Fprintln(w, "Desktop entry:\t(not found)")
	}
	if info.IconFile != "" {
		fmt.Fprintf(w, "Icon file:\t%s\n", info.IconFile)
	}
	if info.IconExtract != "" {
		fmt.Fprintf(w, "Extracted icon:\t%s\n", info.IconExtract)
	}
	if info.Degraded {
		fmt.Fprintln(w, "State:\tdegraded")
	}
	w.Flush()

	for _, wmsg := range info.Warnings {
		log.Warn("%s", wmsg)
	}
	if log.Level() >= logging.LevelVVV {
		log.Debug("origins=%v extractTool=%s", info.Origins, info.ExtractTool)
	}
}
