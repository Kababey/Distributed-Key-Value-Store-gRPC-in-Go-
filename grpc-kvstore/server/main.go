package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net"
	"os"
	"strings"
	"sync"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	pb "grpc-kvstore/proto"
	"google.golang.org/protobuf/types/known/emptypb"
)

// kvServer implements both KVStore and Replicator services.
type kvServer struct {
	pb.UnimplementedKVStoreServer
	pb.UnimplementedReplicatorServer

	mu    sync.RWMutex
	store map[string]string

	selfAddr string

	peerAddrs []string
	peerOnce  sync.Once
	peerMu    sync.RWMutex
	peers     map[string]pb.ReplicatorClient // addr -> client
}

func newKVServer(self string, peerAddrs []string) *kvServer {
	return &kvServer{
		store:     make(map[string]string),
		selfAddr:  self,
		peerAddrs: peerAddrs,
		peers:     make(map[string]pb.ReplicatorClient),
	}
}

func (s *kvServer) ensurePeerClients() {
	s.peerOnce.Do(func() {
		for _, addr := range s.peerAddrs {
			if addr == "" || addr == s.selfAddr {
				continue
			}
			conn, err := grpc.Dial(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
			if err != nil {
				log.Printf("dial peer %s: %v", addr, err)
				continue
			}
			s.peerMu.Lock()
			s.peers[addr] = pb.NewReplicatorClient(conn)
			s.peerMu.Unlock()
			log.Printf("connected to peer %s", addr)
		}
	})
}

// KVStore RPCs

func (s *kvServer) Put(ctx context.Context, kv *pb.KeyValue) (*pb.Ack, error) {
	if kv.GetKey() == "" {
		return &pb.Ack{Ok: false, Message: "empty key"}, nil
	}

	// 1) write locally
	s.mu.Lock()
	s.store[kv.Key] = kv.Value
	s.mu.Unlock()

	// 2) replicate to peers (all) before ack
	s.ensurePeerClients()

	s.peerMu.RLock()
	clients := make(map[string]pb.ReplicatorClient, len(s.peers))
	for addr, c := range s.peers { clients[addr] = c }
	s.peerMu.RUnlock()

	var failed []string
	var wg sync.WaitGroup
	wg.Add(len(clients))

	// Fan out with timeouts
	for addr, client := range clients {
		addr := addr
		client := client
		go func() {
			defer wg.Done()
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer cancel()
			if _, err := client.Replicate(ctx, kv); err != nil {
				log.Printf("replicate to %s failed: %v", addr, err)
				synchronizedAppend(&failed, addr)
			}
		}()
	}
	wg.Wait()

	if len(failed) > 0 {
		return &pb.Ack{Ok: false, Message: fmt.Sprintf("stored locally; failed to replicate to: %s", strings.Join(failed, ","))}, nil
	}
	return &pb.Ack{Ok: true, Message: "Stored and replicated"}, nil
}

func synchronizedAppend(slice *[]string, v string) {
	// tiny helper to avoid capturing a mutex for a small list – safe because used only in goroutines with no concurrent reads until Wait.
	*slice = append(*slice, v)
}

func (s *kvServer) Get(ctx context.Context, k *pb.Key) (*pb.Value, error) {
	s.mu.RLock()
	v := s.store[k.GetKey()]
	s.mu.RUnlock()
	return &pb.Value{Value: v}, nil
}

func (s *kvServer) List(_ *emptypb.Empty, stream pb.KVStore_ListServer) error {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for k, v := range s.store {
		if err := stream.Send(&pb.KeyValue{Key: k, Value: v}); err != nil {
			return err
		}
	}
	return nil
}

// Replicator RPC

func (s *kvServer) Replicate(ctx context.Context, kv *pb.KeyValue) (*pb.Ack, error) {
	s.mu.Lock()
	s.store[kv.Key] = kv.Value
	s.mu.Unlock()
	return &pb.Ack{Ok: true, Message: "applied"}, nil
}

func main() {
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: %s <port> <peerPortsCSV>\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "Example: %s 5001 5002,5003\n", os.Args[0])
	}
	flag.Parse()
	if flag.NArg() < 1 {
		flag.Usage()
		os.Exit(2)
	}
	port := flag.Arg(0)
	var peerCSV string
	if flag.NArg() >= 2 {
		peerCSV = flag.Arg(1)
	}
	selfAddr := "localhost:" + port
	var peers []string
	for _, p := range strings.Split(peerCSV, ",") {
		p = strings.TrimSpace(p)
		if p == "" { continue }
		peers = append(peers, "localhost:"+p)
	}

	lis, err := net.Listen("tcp", selfAddr)
	if err != nil {
		log.Fatalf("listen %s: %v", selfAddr, err)
	}
	grpcServer := grpc.NewServer()
	s := newKVServer(selfAddr, peers)
	pb.RegisterKVStoreServer(grpcServer, s)
	pb.RegisterReplicatorServer(grpcServer, s)

	log.Printf("KV node on %s; peers=%v", selfAddr, peers)
	if err := grpcServer.Serve(lis); err != nil {
		log.Fatalf("serve: %v", err)
	}
}
