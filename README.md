# Distributed Key-Value Store gRPC in Go
A simple distributed key-value store implemented in Go using gRPC, where three server nodes replicate data among themselves for consistency.

A minimal **distributed key–value store** in **Go** using **gRPC**
- Three server nodes (e.g., `5001`, `5002`, `5003`)
- Client can connect to **any** node
- RPCs: `Put(KeyValue)`, `Get(Key)`, `List()` (server-streaming)
- **Inter-node replication** via `Replicate(KeyValue)`; a `Put` is acknowledged only after replication attempts

---

## 📚 Learning Objectives
- Build client–server communication with **gRPC (Go)**  
- Practice **protobuf** definitions + **server-streaming**  
- Implement **replication** and **concurrency-safe** state with mutexes

---

## 🧩 Project Structure
grpc-kvstore\
├─ proto\
│ └─ kvstore.proto\
├─ server\
│ └─ main.go\
└─ client\
└─ main.go 


---

## 📦 Prerequisites

- **Go** ≥ 1.21
- **Protocol Buffers** compiler (`protoc`)
- **Go plugins** for protobuf & gRPC:
  ```bash
  go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
  go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest
  # Ensure Go bin is on PATH (Linux/macOS default)
  export PATH="$(go env GOPATH)/bin:$PATH"

Check that protoc is installed:

```bash

protoc --version
# libprotoc X.Y.Z
```
🛠️ Setup
Clone (or place) the project

```bash
cd ~/grpc-kvstore
```
Initialize Go module & deps
(Keep the module name grpc-kvstore to match imports.)

```bash
go mod init grpc-kvstore
go get google.golang.org/grpc google.golang.org/protobuf
go mod tidy
```
Generate gRPC stubs

```bash
protoc --go_out=. --go_opt=paths=source_relative \
  --go-grpc_out=. --go-grpc_opt=paths=source_relative proto/kvstore.proto
```
🔌 Proto Overview (for reference)
```proto
syntax = "proto3";

package kvstore.v1;
option go_package = "grpc-kvstore/proto;kvstorepb";

import "google/protobuf/empty.proto";

message Key       { string key   = 1; }
message Value     { string value = 1; }
message KeyValue  { string key   = 1; string value = 2; }
message Ack       { bool ok = 1; string message = 2; }

service KVStore {
  rpc Put (KeyValue)                 returns (Ack);
  rpc Get (Key)                      returns (Value);
  rpc List(google.protobuf.Empty)    returns (stream KeyValue);
}

service Replicator {
  rpc Replicate(KeyValue)            returns (Ack);
}
```
🚀 Run
Open three terminals (one per server):

```bash
Copy code
# Terminal 1
go run server/main.go 5001 5002,5003
```
```bash
# Terminal 2
go run server/main.go 5002 5001,5003
```
```bash
# Terminal 3
go run server/main.go 5003 5001,5002
```
Start the client (connect to any node, e.g., 5001):

```bash
Copy code
go run client/main.go 5001
```
🧪 Example Session
```pgsql
> put name Adnan
Stored and replicated
> get name
name = Adnan
> list
name = Adnan
> exit
```
🧠 Implementation Notes

- Data model: in-memory map[string]string per node

- Concurrency: guarded by sync.RWMutex

- Replication path:

  1. Put writes locally

  2. Calls Replicate(KeyValue) on all peers

  3. Acknowledges only after replication attempts (success returns ok: true; partial failures are reported in Ack.message)

- List: server-streaming of all KeyValue pairs


✅ Checklist
 * Launch 3 servers on distinct ports

 * put replicates to all peers

 * get returns the same value from any node

 * list streams all stored pairs

 * Handles concurrent clients safely

🧯 Troubleshooting
no required module provides package ... go.mod file not found
Run:

```bash
go mod init grpc-kvstore
go get google.golang.org/grpc google.golang.org/protobuf
go mod tidy
protoc-gen-go: program not found / protoc-gen-go-grpc: not found
Install plugins and set PATH:
```
```bash
Copy code
go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest
export PATH="$(go env GOPATH)/bin:$PATH"
```


📚 References
gRPC Go Quickstart: https://grpc.io/docs/languages/go/quickstart/
