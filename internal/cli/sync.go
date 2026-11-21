package cli

import (
	"fmt"

	"appman/internal/manager"

	"github.com/spf13/cobra"
)

func (a *App) newSyncCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "sync",
		Short: "Synchronize AppImage integrations",
		RunE:  a.runSync,
	}
	cmd.Flags().BoolVar(&a.dryRun, "dry-run", false, "Show planned changes without applying them")
	cmd.Flags().BoolVar(&a.prune, "prune", false, "Remove orphaned managed desktop entries")
	return cmd
}

func (a *App) newInstallCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:               "install <app>",
		Short:             "Integrate an AppImage with the desktop",
		Args:              cobra.ExactArgs(1),
		ValidArgsFunction: a.completeAppNames,
		RunE:              a.runInstall,
	}
	cmd.Flags().BoolVar(&a.dryRun, "dry-run", false, "Show planned changes without applying them")
	return cmd
}

func (a *App) newRemoveCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:               "remove <app>",
		Short:             "Remove an AppImage desktop integration",
		Args:              cobra.ExactArgs(1),
		ValidArgsFunction: a.completeAppNames,
		RunE:              a.runRemove,
	}
	cmd.Flags().BoolVar(&a.dryRun, "dry-run", false, "Show planned changes without applying them")
	return cmd
}

func (a *App) runSync(cmd *cobra.Command, args []string) error {
	plan, err := manager.BuildPlan(a.cfg)
	if err != nil {
		return err
	}

	printPlan(cmd.OutOrStdout(), plan, a.log)

	if a.dryRun {
		_, _ = fmt.Fprintln(cmd.OutOrStdout(), "\nDry run: no changes were made.")
		return nil
	}

	if err := manager.Apply(plan, a.prune); err != nil {
		return err
	}

	if hasOrphans(plan) && !a.prune {
		a.log.Warn("Orphaned desktop entries detected.")
		a.log.Warn("Run 'appman sync --prune' to remove them.")
	}

	a.log.OK("Synchronization complete.")
	return nil
}

func (a *App) runInstall(cmd *cobra.Command, args []string) error {
	query := args[0]

	plan, err := manager.BuildPlan(a.cfg)
	if err != nil {
		return err
	}

	filtered := filterPlanFor(plan, query, false)
	if len(filtered.Actions) == 0 {
		return fmt.Errorf("no pending integration changes for %q", query)
	}

	printPlan(cmd.OutOrStdout(), filtered, a.log)

	if a.dryRun {
		_, _ = fmt.Fprintln(cmd.OutOrStdout(), "\nDry run: no changes were made.")
		return nil
	}

	if err := manager.Apply(filtered, false); err != nil {
		return err
	}

	a.log.OK("Integration complete for %s.", query)
	return nil
}

func (a *App) runRemove(cmd *cobra.Command, args []string) error {
	query := args[0]

	plan, err := manager.BuildPlan(a.cfg)
	if err != nil {
		return err
	}

	filtered := filterPlanFor(plan, query, true)
	if len(filtered.Actions) == 0 {
		return fmt.Errorf("no managed integration found for %q", query)
	}

	printPlan(cmd.OutOrStdout(), filtered, a.log)

	if a.dryRun {
		_, _ = fmt.Fprintln(cmd.OutOrStdout(), "\nDry run: no changes were made.")
		return nil
	}

	if err := manager.Apply(filtered, true); err != nil {
		return err
	}

	a.log.OK("Removed integration for %s.", query)
	return nil
}
