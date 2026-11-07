package cmd

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/Bisratasaye12/Distributed_Database/coordinator"

	"github.com/spf13/cobra"
)

var nodeCmd = &cobra.Command{
	Use:   "node",
	Short: "Start nodes and manage cluster membership",
	Long: `Start nodes and manage cluster membership.

Each node uses two ports: --bind-addr for Serf gossip, and --comm-port
for both Raft traffic and the HTTP API (they share one listener).`,
	Example: `  # first node bootstraps a new cluster
  distsql node start --node-name=node1 --bind-addr=127.0.0.1:7946 \
      --comm-port=8946 --data-dir=./data1 --bootstrap

  # more nodes join through the first one
  distsql node start --node-name=node2 --bind-addr=127.0.0.1:7947 \
      --comm-port=8947 --data-dir=./data2 \
      --join=http://127.0.0.1:8946 --peers=127.0.0.1:7946

  distsql node list --addr=http://127.0.0.1:8946`,
}

var startCmd = &cobra.Command{
	Use:   "start",
	Short: "Start a node and keep it running until interrupted",
	RunE: func(cmd *cobra.Command, args []string) error {
		bindAddr, _ := cmd.Flags().GetString("bind-addr")
		commPort, _ := cmd.Flags().GetInt("comm-port")
		dataDir, _ := cmd.Flags().GetString("data-dir")
		bootstrap, _ := cmd.Flags().GetBool("bootstrap")
		nodeName, _ := cmd.Flags().GetString("node-name")
		joinAddr, _ := cmd.Flags().GetString("join")
		peers, _ := cmd.Flags().GetStringSlice("peers")

		if nodeName == "" {
			return fmt.Errorf("--node-name is required")
		}
		if bootstrap && joinAddr != "" {
			return fmt.Errorf("use either --bootstrap (new cluster) or --join (existing cluster), not both")
		}

		// Existing Raft state in dataDir is kept, so a restarted node
		// rejoins with its log instead of starting from scratch.
		if err := os.MkdirAll(dataDir, 0755); err != nil {
			return fmt.Errorf("create data directory: %w", err)
		}

		node, err := coordinator.New(coordinator.Config{
			DataDir:           dataDir,
			BindAddr:          bindAddr,
			CommunicationPort: commPort,
			StartJoinAddrs:    peers,
			Bootstrap:         bootstrap,
			NodeName:          nodeName,
			LeaderStartAddr:   joinAddr,
		})
		if err != nil {
			return fmt.Errorf("start node: %w", err)
		}

		httpAddr, _ := node.Config.HTTPAddr()
		fmt.Fprintf(cmd.OutOrStdout(), "node %s running, HTTP API at %s (Ctrl+C to stop)\n", nodeName, httpAddr)

		stop := make(chan os.Signal, 1)
		signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
		<-stop

		fmt.Fprintln(cmd.OutOrStdout(), "shutting down")
		return node.Close()
	},
}

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show runtime and Raft metrics for a node",
	RunE: func(cmd *cobra.Command, args []string) error {
		var metrics map[string]any
		if err := do(http.MethodGet, addrFlag(cmd)+"/metric", nil, &metrics); err != nil {
			return err
		}

		pretty, err := json.MarshalIndent(metrics, "", "  ")
		if err != nil {
			return err
		}
		fmt.Fprintln(cmd.OutOrStdout(), string(pretty))
		return nil
	},
}

var joinCmd = &cobra.Command{
	Use:   "join",
	Short: "Add a running node to the cluster as a voter",
	RunE: func(cmd *cobra.Command, args []string) error {
		nodeID, _ := cmd.Flags().GetString("node-id")
		raftAddr, _ := cmd.Flags().GetString("raft-addr")
		if nodeID == "" || raftAddr == "" {
			return fmt.Errorf("--node-id and --raft-addr are required")
		}

		body := map[string]string{"id": nodeID, "addr": raftAddr}
		if err := do(http.MethodPost, addrFlag(cmd)+"/join", body, nil); err != nil {
			return err
		}
		fmt.Fprintf(cmd.OutOrStdout(), "%s joined the cluster\n", nodeID)
		return nil
	},
}

var leaveCmd = &cobra.Command{
	Use:   "leave",
	Short: "Remove a node from the cluster",
	RunE: func(cmd *cobra.Command, args []string) error {
		nodeID, _ := cmd.Flags().GetString("node-id")
		if nodeID == "" {
			return fmt.Errorf("--node-id is required")
		}

		if err := do(http.MethodPost, addrFlag(cmd)+"/leave", map[string]string{"id": nodeID}, nil); err != nil {
			return err
		}
		fmt.Fprintf(cmd.OutOrStdout(), "%s left the cluster\n", nodeID)
		return nil
	},
}

var listCmd = &cobra.Command{
	Use:   "list",
	Short: "List the nodes in the cluster and which one is leader",
	RunE: func(cmd *cobra.Command, args []string) error {
		servers, err := listServers(addrFlag(cmd))
		if err != nil {
			return err
		}
		printServers(cmd.OutOrStdout(), servers)
		return nil
	},
}

func init() {
	startCmd.Flags().String("bind-addr", "127.0.0.1:7946", "address for Serf gossip")
	startCmd.Flags().Int("comm-port", 8946, "port for Raft and the HTTP API")
	startCmd.Flags().String("data-dir", "data", "directory for the SQLite database and Raft log")
	startCmd.Flags().Bool("bootstrap", false, "start a new cluster with this node as the first member")
	startCmd.Flags().String("node-name", "", "unique node id (required)")
	startCmd.Flags().String("join", "", "HTTP address of a cluster member to join through")
	startCmd.Flags().StringSlice("peers", nil, "Serf addresses of existing members, for gossip")

	joinCmd.Flags().String("node-id", "", "id of the node to add")
	joinCmd.Flags().String("raft-addr", "", "Raft address of the node to add")

	leaveCmd.Flags().String("node-id", "", "id of the node to remove")

	nodeCmd.AddCommand(startCmd, statusCmd, joinCmd, leaveCmd, listCmd)
	rootCmd.AddCommand(nodeCmd)
}
