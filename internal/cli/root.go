// Package cli builds the elara command tree. It wraps the existing service
// entrypoint (cmd/service's run()) rather than reimplementing it — serve is
// wired up as both the root command's default action and the explicit
// `serve` subcommand, so `elara` with no args keeps the exact behavior
// existing Docker/Helm entrypoints rely on.
package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

// VersionInfo carries build-time metadata injected via ldflags (see
// cmd/service/main.go) plus the toolchain/platform info available at
// runtime.
type VersionInfo struct {
	Version   string
	Commit    string
	Date      string
	GoVersion string
	OS        string
	Arch      string
}

// Execute builds the elara command tree and runs it. serve is called for
// both `elara` (no subcommand) and `elara serve`.
func Execute(serve func() error, info VersionInfo) error {
	root := newRootCmd(serve, info)

	if err := root.Execute(); err != nil {
		return fmt.Errorf("execute command: %w", err)
	}

	return nil
}

func newRootCmd(serve func() error, info VersionInfo) *cobra.Command {
	root := &cobra.Command{
		Use:   "elara",
		Short: "Elara configuration management service",
		Long: "Elara is a configuration management service: a Web UI + " +
			"ConnectRPC API + etcd-compatible gRPC API backed by a single " +
			"bbolt file.\n\nRunning elara with no subcommand is equivalent " +
			"to `elara serve`.",
		Args: cobra.NoArgs,
		RunE: func(_ *cobra.Command, _ []string) error {
			return serve()
		},
		// main() does its own error logging (slog) and exit-code handling —
		// cobra must not also print the error or a usage dump on failure.
		SilenceUsage:  true,
		SilenceErrors: true,
	}

	root.AddCommand(newServeCmd(serve))
	root.AddCommand(newVersionCmd(info))

	return root
}

func newServeCmd(serve func() error) *cobra.Command {
	return &cobra.Command{
		Use:   "serve",
		Short: "Run the elara server (HTTP/ConnectRPC + etcd-compatible gRPC)",
		Args:  cobra.NoArgs,
		RunE: func(_ *cobra.Command, _ []string) error {
			return serve()
		},
	}
}

func newVersionCmd(info VersionInfo) *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print version information",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			out := fmt.Sprintf(
				"elara %s\n  commit:     %s\n  built:      %s\n  go version: %s\n  platform:   %s/%s\n",
				info.Version, info.Commit, info.Date, info.GoVersion, info.OS, info.Arch,
			)

			if _, err := fmt.Fprint(cmd.OutOrStdout(), out); err != nil {
				return fmt.Errorf("write version output: %w", err)
			}

			return nil
		},
	}
}
