package main

import (
	"bytes"
	"encoding/gob"
	"io"
	"sync"

	"github.com/hashicorp/raft"
	"github.com/spacemonkeygo/spacelog"
)

type lockFSM struct {
	locks   map[string]bool
	mutexes map[string]*sync.Mutex
	logger  *spacelog.Logger
}

func NewFSM(logger *spacelog.Logger) *lockFSM {
	lfsm := &lockFSM{}
	lfsm.locks = make(map[string]bool)
	lfsm.mutexes = make(map[string]*sync.Mutex)
	lfsm.logger = logger
	return lfsm
}
func (fsm *lockFSM) Apply(log *raft.Log) interface{} {
	msg, err := DecodeMessage(log.Data)
	if err != nil {
		return err
	}
	if msg.Type == LockKey {
		lock, ok := fsm.mutexes[msg.Key]
		if !ok {
			lock = &sync.Mutex{}
			fsm.mutexes[msg.Key] = lock
		}
		lock.Lock()
		fsm.locks[msg.Key] = true
	} else if msg.Type == UnlockKey {
		lock, ok := fsm.mutexes[msg.Key]
		if !ok {
			lock := &sync.Mutex{}
			fsm.mutexes[msg.Key] = lock
		}
		fsm.locks[msg.Key] = false
		lock.Unlock()
	} else {
		fsm.logger.Warn("invalid message type, ignoring")
	}
	return nil
}

func (fsm *lockFSM) Snapshot() (raft.FSMSnapshot, error) {
	return &lockSnap{fsm}, nil
}

func (fsm *lockFSM) Restore(source io.ReadCloser) error {
	var lockRestore map[string]bool
	d := gob.NewDecoder(source)

	err := d.Decode(&lockRestore)
	if err != nil {
		return err
	}
	fsm.locks = lockRestore
	return nil
}

type lockSnap struct {
	fsm *lockFSM
}

func (ls *lockSnap) Persist(sink raft.SnapshotSink) error {
	buf := new(bytes.Buffer)
	encoder := gob.NewEncoder(buf)
	if err := encoder.Encode(ls.fsm.locks); err != nil {
		return err
	}
	_, err := sink.Write(buf.Bytes())
	if err != nil {
		return err
	}
	return sink.Close()
}

func (ls *lockSnap) Release() {
	//noop I thinks?
}
