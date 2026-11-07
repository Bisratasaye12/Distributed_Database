package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "distsql",
	Short: "Run and talk to a distsql cluster",
	Long: `distsql runs nodes of a Raft-replicated SQLite cluster and sends them
SQL over HTTP.

Start a node with "distsql node start", then use "distsql exec" for
writes and "distsql query" for reads. Any node accepts requests; writes
sent to a follower are redirected to the leader.`,
	SilenceUsage:  true,
	SilenceErrors: true,
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, "Error:", err)
		os.Exit(1)
	}
}

func init() {
	defaultAddr := os.Getenv("DISTSQL_ADDR")
	if defaultAddr == "" {
		defaultAddr = "http://127.0.0.1:8946"
	}
	rootCmd.PersistentFlags().String("addr", defaultAddr, "HTTP address of any node in the cluster (env DISTSQL_ADDR)")
}

func addrFlag(cmd *cobra.Command) string {
	addr, _ := cmd.Flags().GetString("addr")
	return addr
}
