package raft

//
// this is an outline of the API that raft must expose to
// the service (or tester). see comments below for
// each of these functions for more details.
//
// rf = Make(...)
//   create a new Raft server.
// rf.Start(command interface{}) (index, term, isleader)
//   start agreement on a new Log entry
// rf.GetState() (term, isLeader)
//   ask a Raft for its current term, and whether it thinks it is leader
// ApplyMsg
//   each time a new entry is committed to the Log, each Raft peer
//   should send an ApplyMsg to the service (or tester)
//   in the same server.
//

import (
	// "fmt"
	"bytes"
	"encoding/gob"
	"math/rand"
	"net/rpc"
	"sync"
	"time"
)

// as each Raft peer becomes aware that successive Log entries are
// committed, the peer should send an ApplyMsg to the service (or
// tester) on the same server, via the applyCh passed to Make().
type ApplyMsg struct {
	Index       int
	Command     interface{}
	UseSnapshot bool   // ignore for Assignment2; only used in Assignment3
	Snapshot    []byte // ignore for Assignment2; only used in Assignment3
}

type Log struct {
	Term    int
	Command interface{}
}

// A Go object implementing a single Raft peer.
type Raft struct { //server related variables defined
	mu             sync.Mutex
	peerAddrs      []string            // list of peer addresses (including self)
	persister      *Persister          // persistence helper
	me             int                 // index into peerAddrs
	rpcClientCache map[int]*rpc.Client // cache of *rpc.Client per peer

	// Your data here.
	// Look at the paper's Figure 2 for a description of what
	// state a Raft server must maintain.

	CurrentTerm int //latest term seen
	VotedFor    int //candidateId that received vote in current term

	isLeader         bool //leader rn?
	totalVotes       int  //total number of votes
	electionTimeout  time.Duration
	heartbeatTimeout time.Duration
	resetChan        chan struct{}
	applyCh          chan ApplyMsg

	Log         []Log
	commitIndex int
	lastApplied int
	nextIndex   []int
	matchIndex  []int
}

// return CurrentTerm and whether this server
// believes it is the leader.
func (rf *Raft) GetState() (int, bool) {
	rf.mu.Lock()
	defer rf.mu.Unlock()
	var term int
	var isleader bool
	// Your code here.
	term = rf.CurrentTerm
	isleader = rf.isLeader
	return term, isleader
}

// save Raft's persistent state to stable storage,
// where it can later be retrieved after a crash and restart.
// see paper's Figure 2 for a description of what should be persistent.
/*
 In real situations, fault tolerance is achieved by saving the state to the non-volatile memory
 on each state update and reading the state back from the non-volatile memory on reboot.
 However, for this assignment, you are not required to write state to non-volatile memory. You
 will use the persister object (provided in persister.go file) to save and restore the state.
 When the tester code calls Make() (in raft.go), it provides a persister object that holds the
 most recent state of the Raft (if any). The Raft initializes its state from the persister and uses
 it to save its state every time a state update happens. You can use readRaftState() and
 SaveRaftState() for reading and writing state to persister object, respectively.
 To save state through the persister object, you will have to serialize the state before calling
 persist() API. Similarly, you will have to de-serialize the state after reading the state through
 the readPersist() API. You will also need to determine the places where storage to
 nonvolatile memory is required and where to read state from the non-volatile memory. You
 can find some example code on how to use persist() and readPersist() in the raft.go file.


 Similar to how the RPC system only sends structure field names that begin with
 upper-case letters, and silently ignores fields whose names start with lower-case
 letters, the GOB encoder you'll use to save persistent state only saves fields whose
 names start with upper case letters. This is a common source of mysterious bugs
 since Go doesn't warn you
*/
func (rf *Raft) persist() {
	// Your code here.
	// Example:
	w := new(bytes.Buffer)
	e := gob.NewEncoder(w)
	rf.mu.Lock()
	e.Encode(rf.Log)
	e.Encode(rf.VotedFor)
	e.Encode(rf.CurrentTerm)
	rf.mu.Unlock()
	data := w.Bytes()
	rf.persister.SaveRaftState(data)
}

// restore previously persisted state.
func (rf *Raft) readPersist(data []byte) {
	// Your code here.
	// Example:
	r := bytes.NewBuffer(data)
	d := gob.NewDecoder(r)
	rf.mu.Lock()
	d.Decode(&rf.Log)
	d.Decode(&rf.VotedFor)
	d.Decode(&rf.CurrentTerm)
	rf.mu.Unlock()

}

// example RequestVote RPC arguments structure.
type RequestVoteArgs struct {
	// Your data here.
	Term         int //candidate's term
	CandidateId  int //id of candidate requesting votes
	LastLogIndex int
	LastLogTerm  int
}

// example RequestVote RPC reply structure.
type RequestVoteReply struct {
	// Your data here.
	Term        int  // CurrentTerm, for candidate to update itself if candidate has lower term
	VoteGranted bool //true means candidate received vote
}

// example RequestVote RPC handler.
func (rf *Raft) RequestVote(args RequestVoteArgs, reply *RequestVoteReply) error {
	rf.mu.Lock()
	defer rf.mu.Unlock()

	if args.Term > rf.CurrentTerm {
		rf.CurrentTerm = args.Term
		rf.isLeader = false
		rf.VotedFor = -1
		go rf.persist()
	}

	reply.Term = rf.CurrentTerm
	if args.Term < rf.CurrentTerm {
		reply.VoteGranted = false
		return nil
	}

	lastLogIndex := len(rf.Log) - 1
	lastLogTerm := rf.Log[lastLogIndex].Term

	if (rf.VotedFor == -1 || rf.VotedFor == args.CandidateId) && (args.LastLogTerm > lastLogTerm || (args.LastLogTerm == lastLogTerm && args.LastLogIndex >= lastLogIndex)) {
		reply.VoteGranted = true
		rf.VotedFor = args.CandidateId
		rf.resetElectionTimer()
	} else {
		reply.VoteGranted = false
	}
	return nil

}

// example code to send a RequestVote RPC to a server.
// server is the index of the target server in rf.peerAddrs[].
// expects RPC arguments in args.
// fills in *reply with RPC reply, so caller should
// pass &reply.
// the types of the args and reply passed to Call() must be
// the same as the types of the arguments declared in the
// handler function (including whether they are pointers).
//
// returns true if the RPC was delivered (best-effort semantics over net/rpc).
//
// if you're having trouble getting RPC to work, check that you've
// capitalized all field names in structs passed over RPC, and
// that the caller passes the address of the reply struct with &, not
// the struct itself.
func (rf *Raft) sendRequestVote(server int, args RequestVoteArgs, reply *RequestVoteReply) bool {
	// retry with small backoff and per-call timeout
	backoff := 25 * time.Millisecond
	for attempt := 0; attempt < 3; attempt++ {
		addr := rf.peerAddrs[server]
		client := rf.getOrDial(server, addr)
		if client == nil {
			time.Sleep(backoff)
			backoff *= 2
			continue
		}
		callDone := make(chan error, 1)
		go func() { callDone <- client.Call("Raft.RequestVote", args, reply) }()
		select {
		case err := <-callDone:
			if err == nil { return true }
			rf.mu.Lock(); delete(rf.rpcClientCache, server); rf.mu.Unlock()
		case <-time.After(200 * time.Millisecond):
			// timeout, evict and backoff
			rf.mu.Lock(); delete(rf.rpcClientCache, server); rf.mu.Unlock()
		}
		time.Sleep(backoff)
		backoff *= 2
	}
	return false
}

type AppendEntriesArgs struct {
	Term         int
	LeaderId     int
	PrevLogIndex int
	PrevLogTerm  int
	//Entries type has to be an array of structs. need to define struct for Log entries
	Entries      []Log
	LeaderCommit int
}

type AppendEntriesReply struct {
	Term    int
	Success bool
}

func (rf *Raft) AppendEntries(args AppendEntriesArgs, reply *AppendEntriesReply) error {
	rf.mu.Lock()
	defer rf.mu.Unlock()
	if args.Term < rf.CurrentTerm {
		reply.Term = rf.CurrentTerm
		reply.Success = false
		return nil
	}
	if args.Term > rf.CurrentTerm { // step down
		rf.CurrentTerm = args.Term
		rf.isLeader = false
		rf.VotedFor = -1 //new term so reset votedfor
		go rf.persist()
	}
	rf.resetElectionTimer()
	reply.Term = rf.CurrentTerm

	if len(args.Entries) == 0 {
		if args.PrevLogIndex >= len(rf.Log) || rf.Log[args.PrevLogIndex].Term != args.PrevLogTerm {
			reply.Success = false
			return nil
		}
		reply.Success = true
	}

	if args.PrevLogIndex >= len(rf.Log) || rf.Log[args.PrevLogIndex].Term != args.PrevLogTerm {
		reply.Success = false
		return nil
	}
	reply.Success = true

	for i := 0; i < len(args.Entries); i++ { //deleting all entries further entries if conflict
		if args.PrevLogIndex+1+i < len(rf.Log) && rf.Log[args.PrevLogIndex+1+i].Term != args.Entries[i].Term {
			rf.Log = rf.Log[:args.PrevLogIndex+1+i]
			break
		}
	}
	rf.Log = append(rf.Log[:args.PrevLogIndex+1], args.Entries...)
	go rf.persist()
	if args.LeaderCommit > rf.commitIndex {
		rf.commitIndex = min(args.LeaderCommit, len(rf.Log)-1)
	}
	reply.Success = true
	return nil
}

func (rf *Raft) sendAppendEntries(server int, args AppendEntriesArgs, reply *AppendEntriesReply) bool {
	backoff := 25 * time.Millisecond
	for attempt := 0; attempt < 3; attempt++ {
		addr := rf.peerAddrs[server]
		client := rf.getOrDial(server, addr)
		if client == nil {
			time.Sleep(backoff)
			backoff *= 2
			continue
		}
		callDone := make(chan error, 1)
		go func() { callDone <- client.Call("Raft.AppendEntries", args, reply) }()
		select {
		case err := <-callDone:
			if err == nil { return true }
			rf.mu.Lock(); delete(rf.rpcClientCache, server); rf.mu.Unlock()
		case <-time.After(200 * time.Millisecond):
			rf.mu.Lock(); delete(rf.rpcClientCache, server); rf.mu.Unlock()
		}
		time.Sleep(backoff)
		backoff *= 2
	}
	return false
}

// the service using Raft (e.g. a k/v server) wants to start
// agreement on the next command to be appended to Raft's Log. if this
// server isn't the leader, returns false. otherwise start the
// agreement and return immediately. there is no guarantee that this
// command will ever be committed to the Raft Log, since the leader
// may fail or lose an election.
//
// the first return value is the index that the command will appear at
// if it's ever committed. the second return value is the current
// term. the third return value is true if this server believes it is
// the leader.
func (rf *Raft) Start(command interface{}) (int, int, bool) { // need to implement completely first. followers return -1,-1, fa lse. Leader will create Log, append it return that Log's loGindex, term and true.
	//command received is an interface object. we will not do anything with that except place it in the Log. the Log is a list of structs.
	// index := -1
	// term := -1
	// isLeader := true

	// return index, term, isLeader
	rf.mu.Lock()
	defer rf.mu.Unlock()

	if !rf.isLeader {
		return -1, -1, false
	}
	newLog := Log{
		Term:    rf.CurrentTerm,
		Command: command,
	}
	rf.Log = append(rf.Log, newLog)
	go rf.persist()
	indx := len(rf.Log) - 1
	rf.matchIndex[rf.me] = indx
	rf.nextIndex[rf.me] = indx + 1

	return indx, rf.CurrentTerm, true
}

// the tester calls Kill() when a Raft instance won't
// be needed again. you are not required to do anything
// in Kill(), but it might be convenient to (for example)
// turn off debug output from this instance.
func (rf *Raft) Kill() {
	// Your code here, if desired.
}

func (rf *Raft) heartbeatSender() {
	for {
		time.Sleep(rf.heartbeatTimeout)
		rf.mu.Lock()
		if rf.isLeader {
			currentTerm := rf.CurrentTerm
			for i := range rf.peerAddrs {
				if i != rf.me {
					var entries []Log
					if rf.nextIndex[i]-1 < len(rf.Log) {
						entries = make([]Log, len(rf.Log[rf.nextIndex[i]:]))
						copy(entries, rf.Log[rf.nextIndex[i]:])
					} else {
						entries = []Log{}
					}
					args := AppendEntriesArgs{Term: currentTerm, LeaderId: rf.me, Entries: entries, LeaderCommit: rf.commitIndex}
					if rf.nextIndex[i]-1 < len(rf.Log) {
						args.PrevLogIndex = rf.nextIndex[i] - 1
						args.PrevLogTerm = rf.Log[args.PrevLogIndex].Term

					}

					go func(server int) {
						var reply AppendEntriesReply
						if rf.sendAppendEntries(server, args, &reply) {
							rf.mu.Lock()
							defer rf.mu.Unlock()
							if reply.Term > rf.CurrentTerm {
								rf.CurrentTerm = reply.Term
								rf.isLeader = false
								rf.VotedFor = -1
								go rf.persist()
								return
							} else if reply.Success {
								rf.nextIndex[server] = args.PrevLogIndex + len(args.Entries) + 1
								rf.matchIndex[server] = rf.nextIndex[server] - 1

								for N := len(rf.Log) - 1; N > rf.commitIndex; N-- {
									count := 1
									for j := range rf.peerAddrs {
										if j != rf.me && rf.matchIndex[j] >= N {
											count++
										}
									}
									if count > len(rf.peerAddrs)/2 && rf.Log[N].Term == rf.CurrentTerm {
										rf.commitIndex = N
										break
									}
								}
							} else {
								if rf.nextIndex[server] > 1 {
									rf.nextIndex[server]--
								}
							}
						}
					}(i)
				}
			}
		}
		rf.mu.Unlock()
	}
}

func (rf *Raft) heartbeatListener() {
	for {
		timeoutChan := time.After(rf.electionTimeout)

		select {
		case <-timeoutChan:
			rf.mu.Lock()
			if rf.isLeader {
				rf.mu.Unlock()
				continue
			}
			rf.CurrentTerm++
			rf.isLeader = false
			rf.VotedFor = rf.me
			go rf.persist()
			rf.totalVotes = 1
			rf.mu.Unlock()

			for i := range rf.peerAddrs {
				if i != rf.me {
					args := RequestVoteArgs{Term: rf.CurrentTerm, CandidateId: rf.me, LastLogIndex: len(rf.Log) - 1, LastLogTerm: rf.Log[len(rf.Log)-1].Term}
					reply := RequestVoteReply{}
					go func(server int) {
						if rf.sendRequestVote(server, args, &reply) {
							rf.mu.Lock()
							defer rf.mu.Unlock()
							if reply.VoteGranted {
								rf.totalVotes++

								if rf.totalVotes > len(rf.peerAddrs)/2 {
									rf.isLeader = true
									lastLogIndex := len(rf.Log) - 1
									for i := range rf.peerAddrs {
										rf.nextIndex[i] = lastLogIndex + 1
										rf.matchIndex[i] = 0
									}

								}

							} else if reply.Term > rf.CurrentTerm {
								rf.CurrentTerm = reply.Term
								rf.isLeader = false
								rf.VotedFor = -1
								go rf.persist()
							}
						}
					}(i)
				}
			}
		case <-rf.resetChan:
			//election timer reset#
		}

	}
}

// commitIndex should be updated by the leader only if there is an entry at the currentTerm that has been copied on the majority of the servers.
func (rf *Raft) applyLogEntries() {
	for {
		time.Sleep(5 * time.Millisecond)
		rf.mu.Lock()
		for rf.commitIndex > rf.lastApplied {
			nextApplyIndex := rf.lastApplied + 1
			if nextApplyIndex < len(rf.Log) {
				msg := ApplyMsg{
					Index:   nextApplyIndex,
					Command: rf.Log[nextApplyIndex].Command,
				}
				rf.lastApplied = nextApplyIndex
				rf.mu.Unlock()
				rf.applyCh <- msg
				rf.mu.Lock()
			} else {

				break
			}
		}
		rf.mu.Unlock()
	}
}

func (rf *Raft) resetElectionTimer() {
	select {
	case rf.resetChan <- struct{}{}:
	default:
	}
}

// the service or tester wants to create a Raft server. the ports
// of all the Raft servers (including this one) are in peers[]. this
// server's port is peers[me]. all the servers' peers[] arrays
// have the same order. persister is a place for this server to
// save its persistent state, and also initially holds the most
// recent saved state, if any. applyCh is a channel on which the
// tester or service expects Raft to send ApplyMsg messages.
// Make() must return quickly, so it should start goroutines
// for any long-running work.
func Make(peers []string, me int,
	persister *Persister, applyCh chan ApplyMsg) *Raft {
	rf := &Raft{}
	rf.peerAddrs = peers
	rf.persister = persister
	rf.me = me
	rf.CurrentTerm = 0
	rf.VotedFor = -1
	rf.isLeader = false
	rf.rpcClientCache = make(map[int]*rpc.Client)

	rf.commitIndex = 0
	rf.lastApplied = 0
	rf.Log = make([]Log, 0)

	rf.Log = append(rf.Log, Log{Term: 0, Command: nil}) //use first index for dummy Log entry? will have to change commitIndex and lastApplied to cater this?
	go rf.persist()
	rf.nextIndex = make([]int, len(peers))
	rf.matchIndex = make([]int, len(peers))
	for i := range rf.peerAddrs {
		rf.nextIndex[i] = 1
		rf.matchIndex[i] = 0
	}
	rf.electionTimeout = time.Duration(400+rand.Intn(200)) * time.Millisecond
	rf.heartbeatTimeout = time.Duration(110) * time.Millisecond
	rf.resetChan = make(chan struct{}, 1)
	rf.applyCh = applyCh

	go rf.heartbeatListener()
	go rf.heartbeatSender()
	go rf.applyLogEntries()
	// initialize from state persisted before a crash
	rf.readPersist(persister.ReadRaftState())

	return rf
}

// getOrDial returns a cached *rpc.Client or attempts a new dial.
// It does not hold the mutex while dialing to avoid blocking other operations.
func (rf *Raft) getOrDial(server int, addr string) *rpc.Client {
	rf.mu.Lock()
	if c, ok := rf.rpcClientCache[server]; ok {
		rf.mu.Unlock()
		return c
	}
	rf.mu.Unlock()
	c, err := rpc.Dial("tcp", addr)
	if err != nil {
		return nil
	}
	rf.mu.Lock()
	rf.rpcClientCache[server] = c
	rf.mu.Unlock()
	return c
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
