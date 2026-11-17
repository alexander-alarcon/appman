package main

import (
	"fmt"
	"os"
	"path/filepath"
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

	rootCmd.AddCommand(scanCmd, listCmd)
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
