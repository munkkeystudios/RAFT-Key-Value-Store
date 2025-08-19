package main

import (
	"encoding/gob"
	"flag"
	"fmt"
	kv "kvstore/kvraft"
	"kvstore/raft"
	"log"
	"net"
	"net/rpc"
	"os"
	"strings"
)

// Simple standalone server launcher.
// Example:
//
//	go run ./cmd/kvserver -id=0 -addr=127.0.0.1:8000 -peers=127.0.0.1:8000,127.0.0.1:8001,127.0.0.1:8002
func main() {
	id := flag.Int("id", -1, "this server's index in peer list")
	addr := flag.String("addr", "", "listening address host:port")
	peersCSV := flag.String("peers", "", "comma-separated peer addresses (including self)")
	dataDir := flag.String("data-dir", "data", "directory to store raft state & snapshots")
	flag.Parse()

	if *id < 0 || *addr == "" || *peersCSV == "" {
		fmt.Fprintf(os.Stderr, "missing required flags: -id -addr -peers\n")
		flag.Usage()
		os.Exit(2)
	}

	peers := strings.Split(*peersCSV, ",")
	if *id >= len(peers) {
		log.Fatalf("id %d out of range peers len=%d", *id, len(peers))
	}
	if peers[*id] != *addr {
		log.Printf("warning: peers[%d]=%s differs from provided -addr %s", *id, peers[*id], *addr)
	}

	gob.Register(kv.Op{})
	persister := raft.MakeFilePersister(fmt.Sprintf("%s-%d", *dataDir, *id))
	server := kv.StartKVServer(peers, *id, persister, -1)

	// register services
	if err := rpc.Register(server); err != nil {
		log.Fatalf("register kv: %v", err)
	}
	if err := rpc.Register(server.Raft()); err != nil {
		log.Fatalf("register raft: %v", err)
	}

	l, err := net.Listen("tcp", *addr)
	if err != nil {
		log.Fatalf("listen on %s: %v", *addr, err)
	}
	log.Printf("kvserver %d listening at %s peers=%v", *id, *addr, peers)
	for {
		conn, err := l.Accept()
		if err != nil {
			log.Printf("accept error: %v", err)
			continue
		}
		go rpc.ServeConn(conn)
	}
}
