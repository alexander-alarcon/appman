package cli

import (
	"fmt"

	"appman/internal/appimage"

	"github.com/spf13/cobra"
)

func (a *App) newInfoCmd() *cobra.Command {
	return &cobra.Command{
		Use:               "info <app>",
		Short:             "Show detailed information about an AppImage",
		Args:              cobra.ExactArgs(1),
		ValidArgsFunction: a.completeAppNames,
		RunE:              a.runInfo,
	}
}

func (a *App) runInfo(cmd *cobra.Command, args []string) error {
	query := args[0]

	res, err := appimage.Scan(a.appDir)
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

	return printInspect(cmd.OutOrStdout(), info, a.log)
}
