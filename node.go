package main

import (
	"fmt"
	"net"
	"os"
	"path/filepath"
	"runtime"
	"time"

	"github.com/hashicorp/raft"
	"github.com/hashicorp/raft-mdb"
	"github.com/spacemonkeygo/spacelog"
)

type Node struct {
	raft   *raft.Raft
	logger *spacelog.Logger
	addr   *net.TCPAddr
}

const (
	baseDefault            = "/tmp/raft_toy"
	tmpDir                 = "tmp/"
	dataDir                = "data/"
	raftDBSize32bit uint64 = 64 * 1024 * 1024       // Limit Raft log to 64MB
	raftDBSize64bit uint64 = 8 * 1024 * 1024 * 1024 // Limit Raft log to 8GB
	snapsMaintained        = 2
)

// ensurePath is used to make sure a path exists
func ensurePath(path string, dir bool) error {
	if !dir {
		path = filepath.Dir(path)
	}
	return os.MkdirAll(path, 0755)
}

func StartNode(bootstrap bool, bind string, seeds []net.Addr, dataDir string, logger *spacelog.Logger) (*Node, error) {
	n := &Node{}
	n.logger = logger
	baseDir := baseDefault
	if dataDir != "" {
		baseDir = dataDir
	}
	conf := raft.DefaultConfig()
	conf.EnableSingleNode = bootstrap
	conf.LogOutput = logger.Writer(10)
	// Create the base state path
	statePath := filepath.Join(baseDir, tmpDir)
	if err := os.RemoveAll(statePath); err != nil {
		return nil, err
	}
	if err := ensurePath(statePath, true); err != nil {
		return nil, err
	}

	// Set the maximum raft size based on 32/64bit. Since we are
	// doing an mmap underneath, we need to limit our use of virtual
	// address space on 32bit, but don't have to care on 64bit.
	dbSize := raftDBSize32bit
	if runtime.GOARCH == "amd64" {
		dbSize = raftDBSize64bit
	}

	// Create the base raft path
	path := filepath.Join(baseDir, dataDir)
	if err := ensurePath(path, true); err != nil {
		return nil, err
	}

	// Create the MDB store for logs and stable storage
	store, err := raftmdb.NewMDBStoreWithSize(path, dbSize)
	if err != nil {
		return nil, err
	}

	// Create the snapshot store
	snapshots, err := raft.NewFileSnapshotStore(path, snapsMaintained, logger.Writer(10))
	if err != nil {
		store.Close()
		return nil, err
	}

	addr, err := getBindAddr(bind)
	if err != nil {
		return nil, err
	}
	trans, err := raft.NewTCPTransport(bind, addr, 3, 10*time.Second, logger.Writer(10))
	if err != nil {
		return nil, err
	}

	// Setup the peer store
	raftPeers := raft.NewJSONPeers(path, trans)

	// Ensure local host is always included if we are in bootstrap mode
	if bootstrap {
		peers, err := raftPeers.Peers()
		if err != nil {
			store.Close()
			return nil, err
		}
		if !raft.PeerContained(peers, trans.LocalAddr()) {
			peers = append(peers, seeds...)
			raftPeers.SetPeers(raft.AddUniquePeer(peers, trans.LocalAddr()))
		}
	}

	// Setup the Raft store
	n.raft, err = raft.NewRaft(conf, NewFSM(logger), store, store, snapshots, raftPeers, trans)
	if err != nil {
		store.Close()
		trans.Close()
		return nil, err
	}

	n.addr = addr
	go n.startLeaderLoop()

	return n, nil
}

// blocks until a lock can be obtained
func (n *Node) Lock(key string) error {
	f := n.raft.Apply(buildLockDoc(key), time.Second*10)

	err := f.Error()
	if err != nil {
		return err
	}

	f.Response()
	return nil
}

func (n *Node) Unlock(key string) error {
	f := n.raft.Apply(buildUnlockDoc(key), time.Second*10)

	err := f.Error()
	if err != nil {
		return err
	}

	f.Response()
	return nil
}

func (n *Node) IsLeader() bool {
	//return n.raft.Leader() == n.addr
	return true
}

func (n *Node) startLeaderLoop() {
	leaderChan := n.raft.LeaderCh()
	for {
		isLeader := <-leaderChan
		if isLeader {
			n.logger.Info("Running as leader")
		} else {
			n.logger.Info("No longer running as leader")
		}
	}
}
func (n *Node) Stop() {
	n.raft.Shutdown()
}

func buildUnlockDoc(key string) []byte {
	return buildDoc(key, 0)
}

func buildLockDoc(key string) []byte {
	return buildDoc(key, 1)
}

func buildDoc(key string, t int) []byte {
	s := fmt.Sprintf("{\"Type\": %d, \"Key\": \"%v\"}", t, key)
	return []byte(s)
}

func getBindAddr(bind string) (*net.TCPAddr, error) {
	return net.ResolveTCPAddr("tcp", bind)
}
