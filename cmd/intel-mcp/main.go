package main

import (
	"context"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/vend-ai/intel-mcp/internal/config"
	"github.com/vend-ai/intel-mcp/internal/ingest"
	"github.com/vend-ai/intel-mcp/internal/mcp"
	"github.com/vend-ai/intel-mcp/internal/store"
)

func main() {
	var dbPath, manifest string
	root := &cobra.Command{Use: "intel-mcp"}
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
			s, err := store.Open(dbPath)
			if err != nil {
				return err
			}
			defer s.Close()
			return mcp.Serve(context.Background(), s)
		},
	}

	root.AddCommand(indexCmd, serveCmd)
	if err := root.Execute(); err != nil {
		os.Exit(1)
	}
}
