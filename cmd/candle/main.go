package main

import (
	"context"
	"fmt"
	"io"
	"os"

	"github.com/spf13/cobra"

	"github.com/noviopenworks/candle/internal/config"
	"github.com/noviopenworks/candle/internal/ingest"
	"github.com/noviopenworks/candle/internal/mcp"
	"github.com/noviopenworks/candle/internal/registry"
	"github.com/noviopenworks/candle/internal/store"
)

type serveFunc func(context.Context, *store.Store) error
type serveScopedFunc func(context.Context, *store.Store, map[int64]bool) error

func main() {
	var dbPath, manifest string
	root := &cobra.Command{Use: "candle"}
	root.PersistentFlags().StringVar(&dbPath, "db", "intel.db", "SQLite database path")
	root.PersistentFlags().StringVar(&manifest, "config", "manifest.yaml", "repo manifest path")

	indexCmd := &cobra.Command{
		Use:   "index",
		Short: "Ingest repo graphs from the manifest into the store",
		RunE: func(cmd *cobra.Command, _ []string) error {
			cfg, err := config.Load(manifest)
			if err != nil {
				return err
			}
			s, err := store.Open(dbPath)
			if err != nil {
				return err
			}
			defer s.Close()
			rep, err := ingest.Run(s, cfg)
			if err != nil {
				return err
			}
			fmt.Printf("indexed=%d skipped=%d\n", rep.Indexed, rep.Skipped)
			for _, w := range rep.Warnings {
				fmt.Fprintln(os.Stderr, "warning:", w)
			}
			return nil
		},
	}

	serveCmd := &cobra.Command{
		Use:   "serve",
		Short: "Run the MCP stdio server",
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runServe(context.Background(), dbPath, manifest, cmd.Flags().Changed("config"), os.Stderr, mcp.Serve, mcp.ServeScoped)
		},
	}

	root.AddCommand(indexCmd, serveCmd)
	if err := root.Execute(); err != nil {
		os.Exit(1)
	}
}

func runServe(ctx context.Context, dbPath, manifest string, explicitConfig bool, stderr io.Writer, serve serveFunc, serveScoped serveScopedFunc) error {
	s, err := store.Open(dbPath)
	if err != nil {
		return err
	}
	defer s.Close()

	scopePath := ""
	if explicitConfig {
		scopePath = manifest
	} else if _, statErr := os.Stat(manifest); statErr == nil {
		scopePath = manifest
	}

	var allowed map[int64]bool
	if scopePath != "" {
		cfg, lerr := config.Load(scopePath)
		if lerr != nil {
			return lerr
		}
		a, warns, berr := registry.BuildScope(s, cfg)
		if berr != nil {
			return berr
		}
		for _, w := range warns {
			fmt.Fprintln(stderr, "scope warning:", w)
		}
		allowed = a
		fmt.Fprintf(stderr, "serving %d configured snapshot(s) from %s\n", len(allowed), scopePath)
	}

	if allowed == nil {
		return serve(ctx, s)
	}
	return serveScoped(ctx, s, allowed)
}
