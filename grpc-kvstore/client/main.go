package main

import (
	"bufio"
	"context"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	pb "grpc-kvstore/proto"
	"google.golang.org/protobuf/types/known/emptypb"
)

func usage() {
	fmt.Println("Commands: put/get/list/exit")
}

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintf(os.Stderr, "Usage: %s <serverPort>\n", os.Args[0])
		os.Exit(2)
	}
	addr := "localhost:" + os.Args[1]

	conn, err := grpc.Dial(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Fatalf("dial: %v", err)
	}
	defer conn.Close()

	c := pb.NewKVStoreClient(conn)
	in := bufio.NewScanner(os.Stdin)

	for {
		fmt.Print("> ")
		if !in.Scan() {
			break
		}
		line := strings.TrimSpace(in.Text())
		if line == "" {
			continue
		}
		parts := strings.Fields(line)
		op := strings.ToLower(parts[0])

		switch op {
		case "exit", "quit":
			return
		case "put":
			if len(parts) < 3 {
				fmt.Println("Usage: put <key> <value>")
				continue
			}
			kv := &pb.KeyValue{Key: parts[1], Value: strings.Join(parts[2:], " ")}
			ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
			ack, err := c.Put(ctx, kv)
			cancel()
			if err != nil {
				fmt.Println("error:", err)
				continue
			}
			if ack.GetOk() {
				fmt.Println("Stored and replicated")
			} else {
				fmt.Println(ack.GetMessage())
			}
		case "get":
			if len(parts) < 2 {
				fmt.Println("Usage: get <key>")
				continue
			}
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			v, err := c.Get(ctx, &pb.Key{Key: parts[1]})
			cancel()
			if err != nil {
				fmt.Println("error:", err)
				continue
			}
			fmt.Printf("%s = %s\n", parts[1], v.GetValue())
		case "list":
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			stream, err := c.List(ctx, &emptypb.Empty{})
			if err != nil {
				cancel()
				fmt.Println("error:", err)
				continue
			}
			for {
				kv, err := stream.Recv()
				if err != nil {
					cancel()
					if strings.Contains(err.Error(), "EOF") {
						break
					}
					fmt.Println("error:", err)
					break
				}
				fmt.Printf("%s = %s\n", kv.GetKey(), kv.GetValue())
			}
		default:
			usage()
		}
	}
}
