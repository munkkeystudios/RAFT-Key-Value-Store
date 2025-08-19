package main

import (
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"strings"
	"kvstore/kvraft"
	"encoding/base64"
)

// Simple CLI for interacting with the cluster.
// Examples:
//   kvclient --servers=127.0.0.1:8000,127.0.0.1:8001 get mykey
//   kvclient --servers=127.0.0.1:8000,127.0.0.1:8001 put mykey "hello"
//   kvclient --servers=127.0.0.1:8000,127.0.0.1:8001 append mykey " world"
func main() {
	servers := flag.String("servers", "", "comma-separated server addresses")
	flag.Parse()
	args := flag.Args()
	if *servers == "" || len(args) < 2 {
		fmt.Fprintf(os.Stderr, "usage: kvclient --servers=a,b,c get|put|append key [value]\n")
		os.Exit(2)
	}

	addrs := strings.Split(*servers, ",")
	ck := raftkv.MakeClerk(addrs)

	op := strings.ToLower(args[0])
	switch op {
	case "get":
		if len(args) != 2 { log.Fatalf("usage: get key") }
		val := ck.Get(args[1])
		fmt.Println(val)
	case "put":
		if len(args) != 3 { log.Fatalf("usage: put key value") }
		ck.Put(args[1], args[2])
	case "append":
		if len(args) != 3 { log.Fatalf("usage: append key value") }
		ck.Append(args[1], args[2])
	case "putfile":
		if len(args) != 3 { log.Fatalf("usage: putfile key path/to/file") }
		path := args[2]
		data, err := ioutil.ReadFile(path)
		if err != nil {
			log.Fatalf("read file: %v", err)
		}
		b64 := base64.StdEncoding.EncodeToString(data)
		ck.Put(args[1], b64)
		fmt.Printf("stored %d bytes at key %s\n", len(data), args[1])
	case "getfile":
		if len(args) != 3 { log.Fatalf("usage: getfile key path/to/output") }
		b64 := ck.Get(args[1])
		if b64 == "" {
			// empty value => write empty file
			if err := ioutil.WriteFile(args[2], []byte{}, 0644); err != nil {
				log.Fatalf("write file: %v", err)
			}
			fmt.Printf("wrote 0 bytes to %s\n", args[2])
			return
		}
		data, err := base64.StdEncoding.DecodeString(b64)
		if err != nil {
			log.Fatalf("decode base64: %v", err)
		}
		if err := ioutil.WriteFile(args[2], data, 0644); err != nil {
			log.Fatalf("write file: %v", err)
		}
		fmt.Printf("wrote %d bytes to %s\n", len(data), args[2])
	default:
		log.Fatalf("unknown op %q. supported: get, put, append, putfile, getfile", op)
	}
}
