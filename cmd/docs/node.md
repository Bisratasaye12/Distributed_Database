# Node Management Commands

The `distributed_database node` commands manage cluster nodes in the distributed database system.

## Commands Overview

### `start`
Start a new database node.

**Flags:**
- `--bind-addr`: Address to bind Serf on (default "127.0.0.1:7946")
- `--comm-port`: Port for Raft and HTTP communication (default 8946)
- `--data-dir`: Directory to store node data (default "data")
- `--bootstrap`: Bootstrap the cluster (required for first node)
- `--node-name`: Unique node identifier
- `--join`: Leader address to join (required for non-bootstrap nodes)

### `status`
Check node status and health.

**Flags:**
- `--http-addr`: HTTP address of the node (required)

### `join`
Join an existing cluster.

**Flags:**
- `--node-id`: Unique identifier for the node (required)
- `--raft-addr`: Address for Raft communication (required)
- `--data-dir`: Directory to store node data (default "data")

### `leave`
Gracefully leave the cluster.

**Flags:**
- `--node-id`: Unique identifier for the node (required)
- `--leader-addr`: Address of the cluster leader (required)
- `--data-dir`: Directory to store node data (default "data")

### `stop`
Stop a running node.

**Flags:**
- `--data-dir`: Directory to store node data (default "data")

## Common Usage Patterns

### Starting a Cluster

1. Start the bootstrap node (first node):
