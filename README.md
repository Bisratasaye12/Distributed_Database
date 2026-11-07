# distsql

[![CI](https://github.com/Bisratasaye12/Distributed_Database/actions/workflows/ci.yml/badge.svg)](https://github.com/Bisratasaye12/Distributed_Database/actions/workflows/ci.yml)

A small distributed SQL database in Go. Every node keeps a full SQLite copy of the data, and writes are replicated with Raft. You can send SQL to any node: writes sent to a follower get redirected to the leader, and reads can be served locally or forced through the leader.

This is a modified version of [nireo/distsql](https://github.com/nireo/distsql), which wrote the storage engine, the Raft state machine and the HTTP API. I built the command-line tool and the Docker setup, and fixed the bugs listed below that showed up once nodes were restarted and run as a real cluster. Both projects are GPL-3.0 (see [LICENSE](LICENSE) and [NOTICE](NOTICE)).

## Try it

```bash
docker compose up -d --build

docker compose exec node1 distsql node list
# NODE ID  ADDRESS     ROLE
# node1    node1:8946  leader
# node3    node3:8946  follower
# node2    node2:8946  follower

docker compose exec node2 distsql exec \
  "CREATE TABLE users (id INTEGER PRIMARY KEY, name TEXT)" \
  "INSERT INTO users(name) VALUES('abebe')"

docker compose exec node3 distsql query "SELECT * FROM users"

docker compose stop node1          # a new leader is elected
docker compose exec node3 distsql exec "INSERT INTO users(name) VALUES('sara')"
docker compose start node1         # node1 catches up from the log
```

## How it works

```
             distsql exec / query  (HTTP, JSON)
                          │
        ┌─────────────────┼─────────────────┐
        ▼                 ▼                 ▼
   ┌─────────┐       ┌─────────┐       ┌─────────┐
   │  node1  │◄─────►│  node2  │◄─────►│  node3  │
   │ leader  │ Raft  │follower │ Raft  │follower │
   └────┬────┘       └────┬────┘       └────┬────┘
     SQLite            SQLite            SQLite
        └──────── Serf gossip (membership) ──┘
```

- **Raft** (hashicorp/raft) orders every write. A statement is applied to SQLite on each node only after a majority has it in their log.
- **HTTP and Raft share one port.** A connection that starts with byte `1` is Raft; anything else is HTTP (cmux).
- **Serf** gossip finds new nodes and adds them to Raft automatically, so a node can join by gossiping with any member.
- **Reads** are local by default and can be stale by a few milliseconds. `--strong` sends the read through the leader.
- **Restarts:** a node rebuilds its SQLite file from the Raft snapshot and log, so it comes back with exactly the committed data.

## CLI

```
distsql node start   start a node (--bootstrap for the first one, --join for the rest)
distsql node list    nodes in the cluster and which one leads
distsql node status  runtime and Raft metrics
distsql node join    add a running node by id and Raft address
distsql node leave   remove a node
distsql exec  SQL…   run writes, one result line per statement
distsql query SQL…   run reads and print a table (--strong for leader reads)
```

`--addr` (or `DISTSQL_ADDR`) picks the node to talk to. `exec` and `query` exit non-zero if any statement fails, so they work in scripts.

Running a cluster without Docker:

```bash
make build
./bin/distsql node start --node-name=node1 --bind-addr=127.0.0.1:7946 --comm-port=8946 --data-dir=./data1 --bootstrap
./bin/distsql node start --node-name=node2 --bind-addr=127.0.0.1:7947 --comm-port=8947 --data-dir=./data2 \
    --join=http://127.0.0.1:8946 --peers=127.0.0.1:7946
```

## Changes from upstream

Bugs I found and fixed. Each has its own commit, and most have a test:

- **Restarted nodes duplicated data.** Raft replays its log on startup, but the old SQLite file was kept, so every write was applied twice. The file is now rebuilt from Raft. (`TestRestartDoesNotReapplyWrites`)
- **Gossip removed nodes from the cluster.** The address was published under the Serf tag `addr` while the reader expected `rpc_addr`, so each gossip join called Raft with an empty address, and Raft removed the server. (`TestJoinWithEmptyAddressKeepsTheServer`)
- **Writes to followers failed.** Followers redirected with `301`, which makes clients resend a POST as a GET without the body. Now `307`. (`TestExecOnFollowerRedirectsWith307`)
- **Nodes couldn't restart.** Creating the `raft` directory failed if it already existed, the BoltDB log was never closed, a restarted bootstrap node gave up after 3 seconds waiting for an election, and a node rejoined at startup even when it was already a member.
- **Containers changed address.** Raft advertised the resolved container IP instead of the configured hostname, so a restarted container was unreachable.
- **Startup failed if a peer was down.** The Serf join is now retried in the background.
- The repo now builds from its own packages. Before, every import pointed back at the upstream module, so local changes had no effect.

Added: the `distsql` CLI (`exec`, `query`, `node start/list/status/join/leave`), the `/servers` endpoint, the Dockerfile and 3-node compose file, and CI.

## Tests

```bash
make test
```

The coordinator tests start real 3-node clusters on localhost. CI runs `go vet`, the full test suite and a Docker build on every push.

## Limits

This is a learning project, not something to put real data in. Every node stores the full dataset, there's no authentication on the HTTP API, and the leader handles every write, so writes don't scale past one machine.
