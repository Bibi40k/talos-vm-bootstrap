package cli

import (
	"fmt"
	"log/slog"
	"os"
	"strings"

	"github.com/spf13/cobra"
)

var (
	logFormat string
	logLevel  string
)

func newRootCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:           "talos-docker-bootstrap",
		Short:         "Talos bootstrap for Ubuntu dev VM (Docker + Talos in Docker)",
		SilenceUsage:  true,
		SilenceErrors: true,
		PersistentPreRunE: func(_ *cobra.Command, _ []string) error {
			_, err := newLogger(logFormat, logLevel)
			return err
		},
	}

	cmd.PersistentFlags().StringVar(&logFormat, "log-format", "text", "Log format: text|json")
	cmd.PersistentFlags().StringVar(&logLevel, "log-level", "info", "Log level: debug|info|warn|error")

	cmd.AddCommand(newBootstrapCmd())
	cmd.AddCommand(newVMDeployCmd())
	cmd.AddCommand(newConfigCmd())
	cmd.AddCommand(newProvisionAndBootstrapCmd())
	cmd.AddCommand(newClusterStatusCmd())
	cmd.AddCommand(newKubeconfigExportCmd())
	cmd.AddCommand(newMountCheckCmd())

	return cmd
}

func Execute() error {
	cmd := newRootCmd()
	if err := cmd.Execute(); err != nil {
		const (
			red    = "\033[31m"
			yellow = "\033[33m"
			cyan   = "\033[36m"
			reset  = "\033[0m"
		)
		if ue, ok := err.(*userError); ok {
			fmt.Fprintf(os.Stderr, "%sError:%s %s\n", red, reset, ue.Error())
			if hint := ue.Hint(); hint != "" {
				fmt.Fprintf(os.Stderr, "%sHint:%s %s%s%s\n", yellow, reset, cyan, hint, reset)
			}
		} else {
			fmt.Fprintf(os.Stderr, "%sError:%s %v\n", red, reset, err)
		}
		return err
	}
	return nil
}

func newLogger(format, level string) (*slog.Logger, error) {
	var lvl slog.Level
	switch strings.ToLower(level) {
	case "debug":
		lvl = slog.LevelDebug
	case "info":
		lvl = slog.LevelInfo
	case "warn":
		lvl = slog.LevelWarn
	case "error":
		lvl = slog.LevelError
	default:
		return nil, fmt.Errorf("invalid --log-level %q (expected: debug|info|warn|error)", level)
	}

	opts := &slog.HandlerOptions{Level: lvl}
	var h slog.Handler
	switch strings.ToLower(format) {
	case "text":
		h = slog.NewTextHandler(os.Stdout, opts)
	case "json":
		h = slog.NewJSONHandler(os.Stdout, opts)
	default:
		return nil, fmt.Errorf("invalid --log-format %q (expected: text|json)", format)
	}

	logger := slog.New(h)
	slog.SetDefault(logger)
	return logger, nil
}
