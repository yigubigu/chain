package state

import (
	"context"
	"encoding/json"

	"chain/errors"
	"chain/log"
)

// TODO(kr): what data type should we really use?

// State is a general-purpose data store designed to accumulate
// and apply replicated updates from a raft log.
// The zero value is an empty State ready to use.
type State struct {
	state        map[string]string
	peers        map[uint64]string // id -> addr
	appliedIndex uint64
	nextNodeID   uint64
}

// SetPeerAddr sets the address for the given peer.
func (s *State) SetPeerAddr(id uint64, addr string) {
	if s.peers == nil {
		s.peers = make(map[uint64]string)
	}
	s.peers[id] = addr
}

// GetPeerAddr gets the current address for the given peer, if set.
func (s *State) GetPeerAddr(id uint64) (addr string) {
	return s.peers[id]
}

// RemovePeerAddr deletes the current address for the given peer if it exists.
func (s *State) RemovePeerAddr(id uint64) {
	if s.peers == nil {
		return
	}
	delete(s.peers, id)
}

// RestoreSnapshot decodes data and overwrites the contents of s.
// It should be called with the retrieved snapshot
// when bootstrapping a new node from an existing cluster
// or when recovering from a file on disk.
func (s *State) RestoreSnapshot(data []byte, index uint64) error {
	if s.peers == nil {
		s.peers = make(map[uint64]string)
	}
	if s.state == nil {
		s.state = make(map[string]string)
	}
	s.appliedIndex = index
	// TODO(kr): figure out a better snapshot encoding
	err := json.Unmarshal(data, s)
	log.Messagef(context.Background(), "decoded snapshot %#v (err %v)", s, err)
	return errors.Wrap(err)
}

// Snapshot returns an encoded copy of s
// suitable for RestoreSnapshot.
func (s *State) Snapshot() ([]byte, uint64, error) {
	log.Messagef(context.Background(), "encoding snapshot %#v", s)
	// TODO(kr): figure out a better snapshot encoding
	data, err := json.Marshal(s)
	return data, s.appliedIndex, errors.Wrap(err)
}

// Apply applies a raft log entry payload to s.
func (s *State) Apply(data []byte, index uint64) error {
	if index < s.appliedIndex {
		return nil
	}
	if s.state == nil {
		s.state = make(map[string]string)
	}
	// TODO(kr): figure out a better entry encoding
	var x interface{}
	err := json.Unmarshal(data, &x)
	if err != nil {
		// An error here indicates a malformed update
		// was written to the raft log. We do version
		// negotiation in the transport layer, so this
		// should be impossible; by this point, we are
		// all speaking the same version.
		return errors.Wrap(err)
	}
	switch x := x.(type) {
	case float64: //json doesn't unmarshal interface to uint64
		if uint64(x) == s.nextNodeID {
			s.nextNodeID++
		}
	case map[string]interface{}:
		for k, v := range x {
			s.state[k] = v.(string)
		}
	}

	s.appliedIndex = index
	return nil
}

// Provisional read operation.
func (s *State) Get(key string) (value string) {
	return s.state[key]
}

// Set encodes a set operation setting key to value.
// The encoded op should be committed to the raft log,
// then it can be applied with Apply.
func Set(key, value string) []byte {
	// TODO(kr): make a way to delete things
	// TODO(kr): we prob need other operations too, like conditional writes
	// TODO(kr): figure out a better entry encoding
	b, _ := json.Marshal(map[string]string{key: value}) // error can't happen
	return b
}

// AppliedIndex returns the raft log index (applied index) of current state
func (s *State) AppliedIndex() uint64 {
	return s.appliedIndex
}

// IDCounter
func (s *State) NextNodeID() uint64 {
	return s.nextNodeID
}

func IncrementNextNodeID(oldID uint64) []byte {
	//good enough for now - no other operation is an integer
	//ideally have a type to represent the instructions
	b, _ := json.Marshal(oldID) //error can't really happen

	return b
}
