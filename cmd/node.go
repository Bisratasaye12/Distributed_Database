package cmd

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"

	"distributed_database/coordinator"

	"github.com/nireo/distsql/pb"

	"github.com/spf13/cobra"
)

var nodeCmd = &cobra.Command{
	Use:   "node",
	Short: "Manage cluster nodes",
	Long: `Manage distributed database cluster nodes.

Commands:
  start       Start a new node (bootstrap node or join existing cluster)
  stop        Stop a running node
  status      Check node status and health
  join        Join an existing cluster
  leave       Gracefully leave the cluster
  list        List all active nodes in the cluster

Examples:
  # Start a bootstrap node (first node)
  distributed_database node start \
      --bind-addr=127.0.0.1:7946 \
      --comm-port=8946 \
      --data-dir=./data1 \
      --node-name=node1 \
      --bootstrap

  # Start additional node and join cluster
  distributed_database node start \
      --bind-addr=127.0.0.1:7947 \
      --comm-port=8947 \
      --data-dir=./data2 \
      --node-name=node2 \
      --join=http://127.0.0.1:8946

  # Check node status
  distributed_database node status --http-addr=http://127.0.0.1:8946

  # Join existing cluster
  distributed_database node join http://127.0.0.1:8946 \
      --node-id=node3 \
      --raft-addr=127.0.0.1:8948 \
      --data-dir=./data3

  # Leave cluster
  distributed_database node leave \
      --node-id=node2 \
      --leader-addr=http://127.0.0.1:8946 \
      --data-dir=./data2

  # Stop node
  distributed_database node stop --data-dir=./data2

  # List all active nodes in the cluster
  distributed_database node list --http-addr=http://127.0.0.1:8946`,
}

// Add this helper function
func cleanRaftDB(dataDir string) error {
	raftDBPath := filepath.Join(dataDir, "raft", "raft.db")
	if _, err := os.Stat(raftDBPath); err == nil {
		// File exists, remove it
		if err := os.Remove(raftDBPath); err != nil {
			return fmt.Errorf("failed to remove raft.db: %v", err)
		}
	}
	return nil
}

var startCmd = &cobra.Command{
	Use:   "start",
	Short: "Start a new node",
	Run: func(cmd *cobra.Command, args []string) {
		bindAddr, _ := cmd.Flags().GetString("bind-addr")
		commPort, _ := cmd.Flags().GetInt("comm-port")
		dataDir, _ := cmd.Flags().GetString("data-dir")
		bootstrap, _ := cmd.Flags().GetBool("bootstrap")
		nodeName, _ := cmd.Flags().GetString("node-name")
		joinAddr, _ := cmd.Flags().GetString("join")

		// Check if data directory exists first
		if _, err := os.Stat(dataDir); os.IsNotExist(err) {
			// Directory doesn't exist, create it
			if err := os.MkdirAll(dataDir, 0755); err != nil {
				log.Fatalf("Failed to create data directory: %v", err)
			}
		} else if err != nil {
			// Some other error occurred while checking
			log.Fatalf("Failed to check data directory: %v", err)
		}

		// Clean raft.db before starting
		if err := cleanRaftDB(dataDir); err != nil {
			log.Fatalf("Failed to clean raft.db: %v", err)
		}

		config := coordinator.Config{
			DataDir:           dataDir,
			BindAddr:          bindAddr,
			CommunicationPort: commPort,
			Bootstrap:         bootstrap,
			NodeName:          nodeName,
			LeaderStartAddr:   joinAddr,
		}

		_, err := coordinator.New(config)
		if err != nil {
			log.Fatalf("Failed to start node: %v", err)
		}

		// Keep the node running
		fmt.Printf("Node started successfully at %s\n", bindAddr)
		select {} // Block forever
	},
}

var stopCmd = &cobra.Command{
	Use:   "stop",
	Short: "Stop a running node",
	Run: func(cmd *cobra.Command, args []string) {
		dataDir, _ := cmd.Flags().GetString("data-dir")
		if err := cleanRaftDB(dataDir); err != nil {
			log.Fatalf("Failed to clean raft.db: %v", err)
		}
		fmt.Println("Node stopped successfully")
	},
}

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Check node status",
	Run: func(cmd *cobra.Command, args []string) {
		httpAddr, _ := cmd.Flags().GetString("http-addr")
		if httpAddr == "" {
			log.Fatal("--http-addr flag is required")
		}

		resp, err := http.Get(fmt.Sprintf("%s/metric", httpAddr))
		if err != nil {
			log.Fatalf("Failed to get node status: %v", err)
		}
		defer resp.Body.Close()

		var status map[string]interface{}
		if err := json.NewDecoder(resp.Body).Decode(&status); err != nil {
			log.Fatalf("Failed to decode status response: %v", err)
		}

		prettyJSON, _ := json.MarshalIndent(status, "", "  ")
		fmt.Printf("Node Status:\n%s\n", string(prettyJSON))
	},
}

var joinCmd = &cobra.Command{
	Use:   "join <address>",
	Short: "Join a node to the cluster",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		address := args[0]
		nodeID, _ := cmd.Flags().GetString("node-id")
		raftAddr, _ := cmd.Flags().GetString("raft-addr")
		dataDir, _ := cmd.Flags().GetString("data-dir")

		if err := cleanRaftDB(dataDir); err != nil {
			log.Fatalf("Failed to clean raft.db: %v", err)
		}

		body := map[string]string{
			"id":   nodeID,
			"addr": raftAddr,
		}

		jsonBody, err := json.Marshal(body)
		if err != nil {
			log.Fatalf("Failed to marshal join request: %v", err)
		}

		resp, err := http.Post(fmt.Sprintf("%s/join", address), "application/json", bytes.NewBuffer(jsonBody))
		if err != nil {
			log.Fatalf("Failed to join cluster: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			log.Fatalf("Failed to join cluster: received status %d", resp.StatusCode)
		}

		fmt.Printf("Node joined cluster at %s successfully\n", address)
	},
}

var leaveCmd = &cobra.Command{
	Use:   "leave",
	Short: "Leave the cluster",
	Run: func(cmd *cobra.Command, args []string) {
		nodeID, _ := cmd.Flags().GetString("node-id")
		leaderAddr, _ := cmd.Flags().GetString("leader-addr")
		dataDir, _ := cmd.Flags().GetString("data-dir")

		if err := cleanRaftDB(dataDir); err != nil {
			log.Fatalf("Failed to clean raft.db: %v", err)
		}

		body := map[string]string{
			"id": nodeID,
		}

		jsonBody, err := json.Marshal(body)
		if err != nil {
			log.Fatalf("Failed to marshal leave request: %v", err)
		}

		resp, err := http.Post(fmt.Sprintf("%s/leave", leaderAddr), "application/json", bytes.NewBuffer(jsonBody))
		if err != nil {
			log.Fatalf("Failed to leave cluster: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			log.Fatalf("Failed to leave cluster: received status %d", resp.StatusCode)
		}

		fmt.Println("Node left the cluster successfully")
	},
}

var listCmd = &cobra.Command{
	Use:   "list",
	Short: "List all active nodes in the cluster",
	Run: func(cmd *cobra.Command, args []string) {
		httpAddr, _ := cmd.Flags().GetString("http-addr")
		if httpAddr == "" {
			log.Fatal("--http-addr flag is required")
		}

		url := fmt.Sprintf("%s/servers", httpAddr)
		log.Printf("Making request to: %s", url)

		resp, err := http.Get(url)
		if err != nil {
			log.Fatalf("Failed to get node list: %v", err)
		}
		defer resp.Body.Close()

		body, _ := io.ReadAll(resp.Body)
		if resp.StatusCode != http.StatusOK {
			log.Printf("Response body: %s", string(body))
			log.Fatalf("Failed to get node list: received status %d", resp.StatusCode)
		}

		var response struct {
			Servers []*pb.Server `json:"servers"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
			log.Fatalf("Failed to decode response: %v", err)
		}

		// Print the node list in a formatted table
		fmt.Printf("\nCluster Nodes:\n")
		fmt.Printf("%-20s %-30s %-10s\n", "NODE ID", "ADDRESS", "ROLE")
		fmt.Printf("%-20s %-30s %-10s\n", "-------", "-------", "----")

		for _, server := range response.Servers {
			role := "follower"
			if server.IsLeader {
				role = "leader"
			}
			fmt.Printf("%-20s %-30s %-10s\n", server.Id, server.RpcAddr, role)
		}
		fmt.Println()
	},
}

func init() {
	// Add flags for start command
	startCmd.Flags().String("bind-addr", "127.0.0.1:7946", "Address to bind Serf on")
	startCmd.Flags().Int("comm-port", 8946, "Port for Raft and HTTP communication")
	startCmd.Flags().String("data-dir", "data", "Directory to store node data")
	startCmd.Flags().Bool("bootstrap", false, "Bootstrap the cluster")
	startCmd.Flags().String("node-name", "", "Unique node identifier")
	startCmd.Flags().String("join", "", "Leader address to join")

	// Add flags for status command
	statusCmd.Flags().String("http-addr", "", "HTTP address of the node")

	// Add flags for join command
	joinCmd.Flags().String("node-id", "", "Unique identifier for the node")
	joinCmd.Flags().String("raft-addr", "", "Address for Raft communication")
	joinCmd.Flags().String("data-dir", "data", "Directory to store node data")

	// Add flags for leave command
	leaveCmd.Flags().String("node-id", "", "Unique identifier for the node")
	leaveCmd.Flags().String("leader-addr", "", "Address of the cluster leader")
	leaveCmd.Flags().String("data-dir", "data", "Directory to store node data")

	// Add flags for stop command
	stopCmd.Flags().String("data-dir", "data", "Directory to store node data")

	// Fix: Register http-addr flag for list command
	listCmd.Flags().String("http-addr", "", "HTTP address of any node in the cluster")

	// Add subcommands to the node command
	nodeCmd.AddCommand(startCmd, stopCmd, statusCmd, joinCmd, leaveCmd, listCmd)

	// Register `node` command with the root command
	rootCmd.AddCommand(nodeCmd)
}
