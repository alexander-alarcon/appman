package cli

import (
	"strings"

	"appman/internal/appimage"
	"appman/internal/config"

	"github.com/spf13/cobra"
)

func (a *App) completeAppNames(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	if len(args) > 0 {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}

	dir := a.appDir
	if f := cmd.Flags().Lookup("dir"); f != nil && f.Value.String() != "" {
		dir = f.Value.String()
	}
	if dir == "" {
		dir = config.Default().General.AppImageDir
	}

	res, err := appimage.Scan(dir)
	if err != nil {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}

	prefix := strings.ToLower(toComplete)
	seen := map[string]bool{}
	var out []string
	for _, img := range res.Images {
		candidates := []string{img.NameGuess, img.Filename}
		for _, c := range candidates {
			if c == "" || seen[c] {
				continue
			}
			if prefix == "" || strings.Contains(strings.ToLower(c), prefix) {
				seen[c] = true
				out = append(out, c)
			}
		}
	}
	return out, cobra.ShellCompDirectiveNoFileComp
}
