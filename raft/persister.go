package raft

// File-backed Persister for Raft state and snapshots.

import (
	"os"
	"path/filepath"
	"sync"
)

type Persister struct {
	mu       sync.Mutex
	dataDir  string
}

// MakeFilePersister creates a Persister that stores data under dataDir.
func MakeFilePersister(dataDir string) *Persister {
	_ = os.MkdirAll(dataDir, 0o755)
	return &Persister{dataDir: dataDir}
}

// Backwards convenience; prefer MakeFilePersister with explicit dir.
func MakePersister() *Persister { return MakeFilePersister("data") }

func (ps *Persister) statePath() string    { return filepath.Join(ps.dataDir, "raft_state.bin") }
func (ps *Persister) snapshotPath() string { return filepath.Join(ps.dataDir, "snapshot.bin") }

func atomicWrite(path string, data []byte) error {
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil { return err }
	// On Windows, replace requires removing target first
	_ = os.Remove(path)
	return os.Rename(tmp, path)
}

func (ps *Persister) SaveRaftState(data []byte) {
	ps.mu.Lock()
	defer ps.mu.Unlock()
	_ = atomicWrite(ps.statePath(), data)
}

func (ps *Persister) ReadRaftState() []byte {
	ps.mu.Lock()
	defer ps.mu.Unlock()
	b, err := os.ReadFile(ps.statePath())
	if err != nil { return nil }
	return b
}

func (ps *Persister) RaftStateSize() int {
	ps.mu.Lock(); defer ps.mu.Unlock()
	fi, err := os.Stat(ps.statePath())
	if err != nil { return 0 }
	return int(fi.Size())
}

func (ps *Persister) SaveSnapshot(snapshot []byte) {
	ps.mu.Lock(); defer ps.mu.Unlock()
	_ = atomicWrite(ps.snapshotPath(), snapshot)
}

func (ps *Persister) ReadSnapshot() []byte {
	ps.mu.Lock(); defer ps.mu.Unlock()
	b, err := os.ReadFile(ps.snapshotPath())
	if err != nil { return nil }
	return b
}

// Copy creates a new Persister initialized with the current contents.
// This keeps compatibility with any legacy harness code that expects Copy().
func (ps *Persister) Copy() *Persister {
	ps.mu.Lock()
	state, _ := os.ReadFile(ps.statePath())
	snap, _ := os.ReadFile(ps.snapshotPath())
	ps.mu.Unlock()
	dir, _ := os.MkdirTemp("", "raft-persister-*")
	np := &Persister{dataDir: dir}
	_ = atomicWrite(filepath.Join(dir, "raft_state.bin"), state)
	_ = atomicWrite(filepath.Join(dir, "snapshot.bin"), snap)
	return np
}
