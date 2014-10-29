package main

import (
	"encoding/json"
)

type MessageType uint8

const (
	UnlockKey MessageType = iota
	LockKey
)

type Message struct {
	Type MessageType
	Key  string
}

func DecodeMessage(buf []byte) (*Message, error) {
	m := &Message{}
	if err := json.Unmarshal(buf, m); err != nil {
		return nil, err
	}
	return m, nil
}
