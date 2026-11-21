package cli

import (
	"fmt"

	"appman/internal/appimage"

	"github.com/spf13/cobra"
)

func (a *App) newScanCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "scan",
		Short: "Discover AppImages in the configured directory",
		RunE:  a.runScan,
	}
}

func (a *App) runScan(cmd *cobra.Command, args []string) error {
	a.log.Info("Scanning for AppImages in %s...", a.appDir)

	res, err := appimage.Scan(a.appDir)
	if err != nil {
		return err
	}

	for _, scanErr := range res.Errors {
		a.log.Warn("%v", scanErr)
	}

	if len(res.Images) == 0 {
		_, _ = fmt.Fprintln(cmd.OutOrStdout(), "No AppImages found.")
		return nil
	}

	_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Found %d AppImages.\n\n", len(res.Images))
	return printImages(cmd.OutOrStdout(), res.Images, a.log)
}
