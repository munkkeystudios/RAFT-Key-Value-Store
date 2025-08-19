package raftkv

import (
	"crypto/rand"
	"math/big"
	"net/rpc"
	"sync"
	"time"
)

type Clerk struct {
	addrs     []string       // server addresses
	clients   map[int]*rpc.Client
	mu        sync.Mutex
	clientID  int64
	requestID int64
	leaderID  int
}

func nrand() int64 { //to generate a unique random number.
	max := big.NewInt(int64(1) << 62)
	bigx, _ := rand.Int(rand.Reader, max)
	x := bigx.Int64()
	return x
}

func MakeClerk(addrs []string) *Clerk {
	ck := &Clerk{
		addrs:    addrs,
		clients:  make(map[int]*rpc.Client),
		clientID: nrand(),
		leaderID: 0,
	}
	return ck
}

// fetch the current value for a key.
// returns "" if the key does not exist.
// keeps trying forever in the face of all other errors.
//
// you can send an RPC with code like this:
// ok := ck.servers[i].Call("RaftKV.Get", &args, &reply)
//
// the types of args and reply (including whether they are pointers)
// must match the declared types of the RPC handler function's
// arguments. and reply must be passed as a pointer.
func (ck *Clerk) Get(key string) string {
	ck.mu.Lock()
	ck.requestID++
	reqID := ck.requestID
	clientID := ck.clientID
	leader := ck.leaderID
	ck.mu.Unlock()

	args := GetArgs{Key: key, ClientID: clientID, RequestID: reqID}

	for {
		ck.mu.Lock(); leader = ck.leaderID; ck.mu.Unlock()
		// try remembered leader first
		if val, ok := ck.tryGetOnServer(leader, &args); ok {
			return val
		}
		// probe others round-robin
		for i := 1; i < len(ck.addrs); i++ {
			probe := (leader + i) % len(ck.addrs)
			if val, ok := ck.tryGetOnServer(probe, &args); ok {
				ck.mu.Lock(); ck.leaderID = probe; ck.mu.Unlock()
				return val
			}
		}
		time.Sleep(50 * time.Millisecond)
	}
}

// shared by Put and Append.
//
// you can send an RPC with code like this:
// ok := ck.servers[i].Call("RaftKV.PutAppend", &args, &reply)
//
// the types of args and reply (including whether they are pointers)
// must match the declared types of the RPC handler function's
// arguments. and reply must be passed as a pointer.
func (ck *Clerk) PutAppend(key string, value string, op string) {
	ck.mu.Lock()
	ck.requestID++
	reqID := ck.requestID
	clientID := ck.clientID
	leader := ck.leaderID
	ck.mu.Unlock()

	args := PutAppendArgs{Key: key, Value: value, Op: op, ClientID: clientID, RequestID: reqID}
	for {
		ck.mu.Lock(); leader = ck.leaderID; ck.mu.Unlock()
		if ok := ck.tryPAOnServer(leader, &args); ok {
			return
		}
		for i := 1; i < len(ck.addrs); i++ {
			probe := (leader + i) % len(ck.addrs)
			if ok := ck.tryPAOnServer(probe, &args); ok {
				ck.mu.Lock(); ck.leaderID = probe; ck.mu.Unlock()
				return
			}
		}
		time.Sleep(50 * time.Millisecond)
	}
}

// tryGetOnServer attempts a Get RPC to a specific server index.
// Returns (value, success).
func (ck *Clerk) tryGetOnServer(idx int, args *GetArgs) (string, bool) {
	client := ck.getOrDial(idx)
	if client == nil { return "", false }
	reply := GetReply{}
	callDone := make(chan error, 1)
	go func() { callDone <- client.Call("RaftKV.Get", args, &reply) }()
	select {
	case err := <-callDone:
		if err != nil { ck.evict(idx); return "", false }
	case <-time.After(500 * time.Millisecond):
		ck.evict(idx)
		return "", false
	}
	if reply.WrongLeader { return "", false }
	if reply.Err == OK { return reply.Value, true }
	if reply.Err == ErrNoKey { return "", true }
	return "", false
}

// tryPAOnServer attempts PutAppend.
// Returns success.
func (ck *Clerk) tryPAOnServer(idx int, args *PutAppendArgs) bool {
	client := ck.getOrDial(idx)
	if client == nil { return false }
	reply := PutAppendReply{}
	callDone := make(chan error, 1)
	go func() { callDone <- client.Call("RaftKV.PutAppend", args, &reply) }()
	select {
	case err := <-callDone:
		if err != nil { ck.evict(idx); return false }
	case <-time.After(500 * time.Millisecond):
		ck.evict(idx)
		return false
	}
	if reply.WrongLeader { return false }
	if reply.Err == OK { return true }
	return false
}

func (ck *Clerk) getOrDial(idx int) *rpc.Client {
	ck.mu.Lock()
	if c, ok := ck.clients[idx]; ok {
		ck.mu.Unlock(); return c
	}
	addr := ck.addrs[idx]
	ck.mu.Unlock()
	c, err := rpc.Dial("tcp", addr)
	if err != nil { return nil }
	ck.mu.Lock()
	ck.clients[idx] = c
	ck.mu.Unlock()
	return c
}

func (ck *Clerk) evict(idx int) {
	ck.mu.Lock(); defer ck.mu.Unlock()
	if c, ok := ck.clients[idx]; ok { c.Close(); delete(ck.clients, idx) }
}

func (ck *Clerk) Put(key string, value string) {
	ck.PutAppend(key, value, "Put")
}
func (ck *Clerk) Append(key string, value string) {
	ck.PutAppend(key, value, "Append")
}
