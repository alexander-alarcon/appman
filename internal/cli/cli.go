package cli

import (
	"fmt"
	"io"
	"os"

	"appman/internal/config"
	"appman/internal/logging"

	"github.com/spf13/cobra"
)

type App struct {
	Out io.Writer
	Err io.Writer

	verbosity int
	appDir    string
	log       *logging.Logger
	cfg       config.Config

	dryRun bool
	prune  bool

	listInstalled bool
	listAvailable bool
	listOrphaned  bool
}

func New() *App {
	return &App{
		Out: os.Stdout,
		Err: os.Stderr,
	}
}

func Execute() error {
	return New().Execute()
}

func (a *App) Execute() error {
	defaultCfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(a.Err, "error: loading configuration: %v\n", err)
		return err
	}

	rootCmd := a.newRootCmd(defaultCfg)
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(a.Err, "error: %v\n", err)
		return err
	}
	return nil
}

func (a *App) newRootCmd(defaultCfg config.Config) *cobra.Command {
	rootCmd := &cobra.Command{
		Use:           "appman",
		Short:         "Manage your AppImages",
		SilenceUsage:  true,
		SilenceErrors: true,
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			a.log = logging.New(logging.LevelFromCount(a.verbosity))
			c, err := config.Load()
			if err != nil {
				return fmt.Errorf("loading configuration: %w", err)
			}
			a.cfg = c
			if cmd.Flags().Changed("dir") {
				a.cfg.General.AppImageDir = a.appDir
			} else {
				a.appDir = a.cfg.General.AppImageDir
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Fprintln(cmd.OutOrStdout(), "No command specified. Run 'appman --help' to see available commands.")
			return nil
		},
	}

	rootCmd.SetOut(a.Out)
	rootCmd.SetErr(a.Err)

	rootCmd.PersistentFlags().CountVarP(&a.verbosity, "verbose", "v", "Increase verbosity (-v, -vv, -vvv)")
	rootCmd.PersistentFlags().StringVar(&a.appDir, "dir", defaultCfg.General.AppImageDir, "Directory containing AppImages")

	rootCmd.AddCommand(
		a.newScanCmd(),
		a.newListCmd(),
		a.newInfoCmd(),
		a.newSyncCmd(),
		a.newInstallCmd(),
		a.newRemoveCmd(),
	)
	return rootCmd
}
