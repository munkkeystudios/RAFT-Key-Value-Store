package raftkv

import (
	"encoding/gob"
	"log"
	"kvstore/raft"
	"sync"
	"time"
)

const Debug = 0

func DPrintf(format string, a ...interface{}) (n int, err error) {
	if Debug > 0 {
		log.Printf(format, a...)
	}
	return
}

type Op struct {
	// Your definitions here.
	// Field names must start with capital letters,
	// otherwise RPC will break.
	Key       string
	Value     string
	Op        string
	ClientID  int64
	RequestID int64
}

type RaftKV struct {
	mu      sync.Mutex
	me      int //server id
	rf      *raft.Raft
	applyCh chan raft.ApplyMsg

	maxraftstate int // snapshot if log grows this big  //ignore this for now

	// Your definitions here.
	kvStore       map[string]string //store the key value pairs
	clientRequest map[int64]int64   //store the client id and the request id
	pendingReqs   map[int]chan Op   //store the pending requests
}

// Raft returns the underlying raft instance (needed for RPC registration).
func (kv *RaftKV) Raft() *raft.Raft { return kv.rf }

func (kv *RaftKV) Get(args *GetArgs, reply *GetReply) error {
	// Your code here.
	op := Op{args.Key, "", "Get", args.ClientID, args.RequestID}
	index, _, isLeader := kv.rf.Start(op)
	if !isLeader {
		reply.WrongLeader = true
		return nil
	}
	kv.mu.Lock()
	ch := make(chan Op, 1)
	kv.pendingReqs[index] = ch

	kv.mu.Unlock()
	defer func() {
		kv.mu.Lock()
		delete(kv.pendingReqs, index)
		kv.mu.Unlock()
	}()
	select {
	case appliedOp := <-ch:
		if appliedOp.ClientID == op.ClientID && appliedOp.RequestID == op.RequestID {
			kv.mu.Lock()
			value, exists := kv.kvStore[args.Key]
			kv.mu.Unlock()
			reply.WrongLeader = false
			if exists {
				reply.Err = OK
				reply.Value = value
			} else {
				reply.Err = ErrNoKey
			}
		} else {
			reply.WrongLeader = true
		}
case <-time.After(500 * time.Millisecond):
	reply.WrongLeader = true
}
return nil
}

func (kv *RaftKV) PutAppend(args *PutAppendArgs, reply *PutAppendReply) error {
	// Your code here.
	op := Op{args.Key, args.Value, args.Op, args.ClientID, args.RequestID}
	index, _, isLeader := kv.rf.Start(op)
	if !isLeader {
		reply.WrongLeader = true
		return nil
	}
	kv.mu.Lock()
	ch := make(chan Op, 1)
	kv.pendingReqs[index] = ch
	kv.mu.Unlock()
	defer func() {
		kv.mu.Lock()
		delete(kv.pendingReqs, index)
		kv.mu.Unlock()
	}()
	select {
	case appliedOp := <-ch:
		if appliedOp.ClientID == op.ClientID && appliedOp.RequestID == op.RequestID {
			reply.WrongLeader = false
			reply.Err = OK
		} else {
			reply.WrongLeader = true
		}
case <-time.After(500 * time.Millisecond):
	reply.WrongLeader = true
}
return nil
}

func (kv *RaftKV) applyRecv() {
	for {
		msg := <-kv.applyCh
		if msg.Command != nil {
			op := msg.Command.(Op)
			kv.mu.Lock()
			lastReqID, exists := kv.clientRequest[op.ClientID]
			if !exists || lastReqID < op.RequestID {
				if op.Op == "Put" {
					kv.kvStore[op.Key] = op.Value
				} else if op.Op == "Append" {
					kv.kvStore[op.Key] += op.Value
				}
				kv.clientRequest[op.ClientID] = op.RequestID
			}
			if ch, ok := kv.pendingReqs[msg.Index]; ok {
				select {
				case ch <- op:
				default:
				}
			}
			kv.mu.Unlock()
		}
	}
}

// the tester calls Kill() when a RaftKV instance won't
// be needed again. you are not required to do anything
// in Kill(), but it might be convenient to (for example)
// turn off debug output from this instance.
func (kv *RaftKV) Kill() {
	kv.rf.Kill()
	// Your code here, if desired.
}

// servers[] contains the ports of the set of
// servers that will cooperate via Raft to
// form the fault-tolerant key/value service.
// me is the index of the current server in servers[].
// the k/v server should store snapshots with persister.SaveSnapshot(),
// and Raft should save its state (including log) with persister.SaveRaftState().
// the k/v server should snapshot when Raft's saved state exceeds maxraftstate bytes,
// in order to allow Raft to garbage-collect its log. if maxraftstate is -1,
// you don't need to snapshot.
// StartKVServer() must return quickly, so it should start goroutines
// for any long-running work.
// StartKVServer creates a new RaftKV backed by a Raft instance.
// peers: slice of peer addresses (including this server's address)
// me: index of this server in peers
func StartKVServer(peers []string, me int, persister *raft.Persister, maxraftstate int) *RaftKV {
	// call gob.Register on structures you want
	// Go's RPC library to marshall/unmarshall.
	gob.Register(Op{})

	kv := new(RaftKV)
	kv.me = me
	kv.maxraftstate = maxraftstate

	// Your initialization code here.
	kv.kvStore = make(map[string]string)
	kv.clientRequest = make(map[int64]int64)
	kv.pendingReqs = make(map[int]chan Op)

	kv.applyCh = make(chan raft.ApplyMsg)
	kv.rf = raft.Make(peers, me, persister, kv.applyCh)

	go kv.applyRecv()
	return kv
}
