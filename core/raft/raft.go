// Package raft provides a simple key-value store coordinated
// across a raft cluster.
package raft

// tktk need to vendor raft package before landing this

import (
	"bytes"
	"context"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"hash/crc32"
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/coreos/etcd/raft"
	"github.com/coreos/etcd/raft/raftpb"
	"github.com/coreos/etcd/snap/snappb"
	"github.com/coreos/etcd/wal"
	"github.com/coreos/etcd/wal/walpb"

	"chain/errors"
	"chain/log"
)

// TODO(kr): do we need a "client" mode?
// So we can have many coreds without all of them
// having to be active raft nodes.
// (Raft isn't really meant for more than a handful
// of consensus participants.)

const (
	tickDur        = 100 * time.Millisecond
	maxRaftReqSize = 10e6 // 10MB
	snapCount      = 10000

	nSnapCatchupEntries uint64 = 10000

	contentType = "application/octet-stream"
)

var crcTable = crc32.MakeTable(crc32.Castagnoli)

// Service holds the key-value data and performs raft coordination.
type Service struct {
	// config
	dir string
	id  uint64
	mux *http.ServeMux

	errMu sync.Mutex
	err   error

	confChangeID uint64 // atomic access only

	// The storage object is purely for internal use
	// by the raft.Node to maintain consistent persistent state.
	// All client code accesses the cluster state via our Service
	// object, which keeps a local, in-memory copy of the
	// complete current state.
	raftNode    raft.Node
	raftStorage *raft.MemoryStorage

	// The actual replicated data set
	// and its current log position.
	// TODO(kr): maybe merge this with raftStorage to save memory?
	// TODO(kr): what data type should we really use?
	stateMu sync.Mutex
	state   map[string]string
	peers   map[uint64]string // id -> addr
	// TODO(kr): figure out synchronization for peers (should it even be under stateMu?)
	// ...also, do we need to replicate it along with the snapshot?

	// access only from runUpdates goroutine
	appliedIndex uint64
	snapIndex    uint64
	confState    raftpb.ConfState
}

// We need to persist and restore the peer address list,
// not just the application state.
type snapshot struct {
	State map[string]string // matches Service.state
	Peers map[uint64]string // matches Service.peers
}

// Getter gets a value from a key-value store.
type Getter interface {
	Get(key string) (value string)
}

// Start starts the raft algorithm.
//
// Param laddr is the local address,
// to be used by peers to send messages to the local node.
// The returned *Service handles HTTP requests
// matching the ServeMux pattern /raft/.
// The caller is responsible for registering this handler
// to receive incoming requests on laddr. For example:
//   rs, err := raft.Start(addr, ...)
//   ...
//   http.Handle("/raft/", rs)
//   http.ListenAndServe(addr, nil)
//
// Param dir is the filesystem location for all persistent storage
// for this raft node. If it doesn't exist, Start will create it.
// It has three entries:
//   id    file containing the node's member id (never changes)
//   snap  file containing the last complete state snapshot
//   wal   dir containing the write-ahead log
//
// Param bootURL gives the location of an existing cluster
// for the local process to join.
// It can be either the concrete address of any
// single cluster member or it can point to a load balancer
// for the whole cluster, if one exists.
// An empty bootURL means to start a fresh empty cluster.
// It is ignored when recovering from existing state in dir.
func Start(laddr, dir, bootURL string) (*Service, error) {
	ctx := context.Background()

	sv := &Service{
		dir:         dir,
		peers:       make(map[uint64]string),
		mux:         http.NewServeMux(),
		raftStorage: raft.NewMemoryStorage(),
	}
	// TODO(kr): grpc
	sv.mux.HandleFunc("/raft/join", sv.serveJoin)
	sv.mux.HandleFunc("/raft/msg", sv.serveMsg)

	walobj, err := sv.recover()
	if err != nil {
		return nil, err
	}

	if walobj != nil {
		sv.id, err = readID(sv.dir)
		if err != nil {
			return nil, errors.Wrap(err)
		}
	} else if bootURL != "" {
		err = sv.join(laddr, bootURL) // sets sv.id and state
		if err != nil {
			return nil, err
		}
	} else {
		// brand new cluster!
		sv.id = 1
		sv.state = make(map[string]string)
	}

	c := &raft.Config{
		ID:              sv.id,
		ElectionTick:    10,
		HeartbeatTick:   1,
		Storage:         sv.raftStorage,
		Applied:         sv.appliedIndex,
		MaxSizePerMsg:   4096,
		MaxInflightMsgs: 256,
	}

	if walobj != nil {
		sv.raftNode = raft.RestartNode(c)
	} else if bootURL != "" {
		log.Write(ctx, "raftid", c.ID)
		err = writeID(sv.dir, c.ID)
		if err != nil {
			return nil, err
		}
		walobj, err = wal.Create(sv.walDir(), nil)
		if err != nil {
			return nil, errors.Wrap(err)
		}
		sv.raftNode = raft.RestartNode(c)
	} else {
		log.Write(ctx, "raftid", c.ID)
		err = writeID(sv.dir, c.ID)
		if err != nil {
			return nil, err
		}
		walobj, err = wal.Create(sv.walDir(), nil)
		if err != nil {
			return nil, errors.Wrap(err)
		}
		sv.raftNode = raft.StartNode(c, []raft.Peer{{ID: 1, Context: []byte(laddr)}})
	}

	go sv.runUpdates(walobj)
	go runTicks(sv.raftNode)
	return sv, nil
}

// Err returns a serious error preventing this process from
// operating normally or making progress, if any.
// Note that it is possible to recover from some errors
// returned by Err.
func (sv *Service) Err() error {
	return sv.err
}

// runUpdates runs forever, reading and processing updates from raft
// onto local storage.
func (sv *Service) runUpdates(wal *wal.WAL) {
	for rd := range sv.raftNode.Ready() {
		wal.Save(rd.HardState, rd.Entries)
		if !raft.IsEmptySnap(rd.Snapshot) {
			sv.redo(func() error {
				return sv.saveSnapshot(&rd.Snapshot)
			})
			sv.redo(func() error {
				// Note: wal.SaveSnapshot saves the snapshot *position*,
				// not the actual full snapshot data.
				// That happens in sv.saveSnapshot just above.
				// (So don't worry, we're not saving it twice.)
				return wal.SaveSnapshot(walpb.Snapshot{
					Index: rd.Snapshot.Metadata.Index,
					Term:  rd.Snapshot.Metadata.Term,
				})
			})
			sv.redo(func() error {
				return wal.ReleaseLockTo(rd.Snapshot.Metadata.Index)
			})
			// Only error here is snapshot too old;
			// should be impossible.
			// (And if it happens, it's permanent.)
			sv.redo(func() error {
				return sv.raftStorage.ApplySnapshot(rd.Snapshot)
			})
			sv.snapIndex = rd.Snapshot.Metadata.Index
			sv.confState = rd.Snapshot.Metadata.ConfState
		}
		sv.raftStorage.Append(rd.Entries)

		for _, entry := range rd.CommittedEntries {
			sv.redo(func() error {
				return sv.applyEntry(entry)
			})
		}

		// NOTE(kr): we must apply entries before sending messages,
		// because some ConfChangeAddNode entries contain the address
		// needed for subsequent messages.
		sv.redo(func() error {
			return sv.send(rd.Messages)
		})

		if sv.appliedIndex-sv.snapIndex > snapCount {
			sv.redo(func() error {
				return sv.triggerSnapshot()
			})
		}
		sv.raftNode.Advance()

		if sv.id == 1 && len(rd.Entries) > 0 && rd.Entries[len(rd.Entries)-1].Index <= 2 {
			// Don't wait for ElectionTick ticks before becoming first leader
			// in a fresh single-node cluster; do it immediately.
			err := sv.raftNode.Campaign(context.Background())
			if err != nil {
				// oh well, we will campaign anyway after ElectionTick
				log.Error(context.Background(), err)
			}
		}

	}
}

func runTicks(rn raft.Node) {
	for range time.Tick(tickDur) {
		rn.Tick()
	}
}

const (
	propWrite = iota
	propRead
)

// Set sets a value in the key-value storage.
// If successful, it returns after the value is committed to
// the raft log.
func (sv *Service) Set(ctx context.Context, key, val string) error {
	// TODO(kr): make a way to delete things
	// TODO(kr): we prob need other operations too, like conditional writes
	b, _ := json.Marshal(map[string]string{key: val}) // error can't happen
	prop := make([]byte, 1+len(b))
	copy(prop[1:], b)

	// TODO(kr): wait for commit
	return errors.Wrap(sv.raftNode.Propose(ctx, prop)) // DOES NOT block for raft
}

// Get gets a value from the key-value store.
// It is linearizable; that is, if a
// Set happens before a Get,
// the Get will observe the effects of the Set.
// (There is still no guarantee an intervening
// Set won't have changed the value again, but it is
// guaranteed not to read stale data.)
// This can be slow; for faster but possibly stale reads, see Stale.
func (sv *Service) Get(key string) string {
	ctx := context.TODO()
	// TODO(kr): piggyback on write ops -- see raft.Node.ReadIndex
	sv.raftNode.Propose(ctx, []byte{propRead})
	// TODO(kr): wait for commit
	return sv.Stale().Get(key)
}

// Stale returns an object that reads
// directly from local memory, returning (possibly) stale data.
// Calls to sv.Get are linearizable,
// which requires them to go through the raft protocol.
// The stale getter skips this, so it is much faster,
// but it can only be used in situations that don't require
// linearizability.
func (sv *Service) Stale() Getter {
	return (*staleGetter)(sv)
}

// ServeHTTP responds to raft consensus messages at /raft/x,
// where x is any particular raft internode RPC.
// When sv sends outgoing messages, it acts as an HTTP client
// and sends requests to its peers at /raft/x.
func (sv *Service) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	sv.mux.ServeHTTP(w, req)
}

func (sv *Service) serveMsg(w http.ResponseWriter, req *http.Request) {
	b, err := ioutil.ReadAll(http.MaxBytesReader(w, req.Body, maxRaftReqSize))
	if err != nil {
		http.Error(w, "cannot read req: "+err.Error(), 400)
		return
	}
	var m raftpb.Message
	err = m.Unmarshal(b)
	if err != nil {
		http.Error(w, "cannot unmarshal: "+err.Error(), 400)
		return
	}
	sv.raftNode.Step(req.Context(), m)
}

func (sv *Service) serveJoin(w http.ResponseWriter, req *http.Request) {
	var x struct{ Addr string }
	err := json.NewDecoder(req.Body).Decode(&x)
	if err != nil {
		http.Error(w, err.Error(), 400)
		return
	}

	log.Write(req.Context(), "at", "join", "addr", x.Addr)

	// TODO(kr): we need to ensure a node id is never reused,
	// and this isn't sufficient. What if the max node got
	// removed? We need a monotonic counter somewhere.
	// (In the application state?)
	// Also, this is racy when concurrent joins happen.
	var newID uint64 = 2
	for _, id := range sv.confState.Nodes {
		if id >= newID {
			newID = id + 1
		}
	}

	log.Write(req.Context(), "at", "join-id", "addr", x.Addr, "id", newID)

	err = sv.raftNode.ProposeConfChange(req.Context(), raftpb.ConfChange{
		ID:      atomic.AddUint64(&sv.confChangeID, 1),
		Type:    raftpb.ConfChangeAddNode,
		NodeID:  newID,
		Context: []byte(x.Addr),
	})
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}

	log.Write(req.Context(), "at", "join-done", "addr", x.Addr, "id", newID)

	snap, err := sv.getSnapshot()
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	snapData, err := encodeSnapshot(snap)
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	json.NewEncoder(w).Encode(struct {
		ID   uint64
		Snap []byte
	}{newID, snapData})
}

// join attempts to join the cluster.
// It requests an existing member to propose a configuration change
// adding the local process as a new member, then retrieves its new ID
// and a snapshot of the cluster state and applies it to sv.
// It also sets sv.id.
func (sv *Service) join(addr, baseURL string) error {
	reqURL := strings.TrimRight(baseURL, "/") + "/raft/join"
	b, _ := json.Marshal(struct{ Addr string }{addr})
	resp, err := http.Post(reqURL, contentType, bytes.NewReader(b))
	if err != nil {
		return errors.Wrap(err)
	}
	defer resp.Body.Close()

	var x struct {
		ID   uint64
		Snap []byte
	}
	err = json.NewDecoder(resp.Body).Decode(&x)
	if err != nil {
		return errors.Wrap(err)
	}
	sv.id = x.ID
	var raftSnap raftpb.Snapshot
	err = decodeSnapshot(x.Snap, &raftSnap)
	if err != nil {
		return errors.Wrap(err)
	}

	ctx := context.Background()
	sv.raftStorage.ApplySnapshot(raftSnap)
	if raft.IsEmptySnap(raftSnap) {
		sv.state = make(map[string]string)
		log.Write(ctx, "at", "joined", "appliedindex", 0, "msg", "(empty snap)")
	} else {
		var snap snapshot
		// TODO(kr): figure out a better snapshot encoding
		err = json.Unmarshal(raftSnap.Data, &snap)
		if err != nil {
			return errors.Wrap(err)
		}
		log.Messagef(ctx, "decoded snapshot %#v", snap)
		sv.state = snap.State
		sv.peers = snap.Peers
		sv.confState = raftSnap.Metadata.ConfState
		sv.snapIndex = raftSnap.Metadata.Index
		sv.appliedIndex = raftSnap.Metadata.Index
		log.Write(ctx, "at", "joined", "appliedindex", sv.appliedIndex)
	}

	return nil
}

func encodeSnapshot(snapshot *raftpb.Snapshot) ([]byte, error) {
	b, err := snapshot.Marshal()
	if err != nil {
		return nil, err
	}

	crc := crc32.Checksum(b, crcTable)
	snap := snappb.Snapshot{Crc: crc, Data: b}
	return snap.Marshal()
}

func decodeSnapshot(data []byte, snapshot *raftpb.Snapshot) error {
	var snapPB snappb.Snapshot
	err := snapPB.Unmarshal(data)
	if err != nil {
		return errors.Wrap(err)
	}
	if crc32.Checksum(snapPB.Data, crcTable) != snapPB.Crc {
		return errors.Wrap(errors.New("bad snapshot crc"))
	}
	err = snapshot.Unmarshal(snapPB.Data)
	return errors.Wrap(err)
}

// TODO(kr): under what circumstances should we do this?
func (sv *Service) evict(nodeID uint64) {
	//cc := raftpb.ConfChange{
	//	Type:   raftpb.ConfChangeRemoveNode,
	//	NodeID: nodeID,
	//}
}

func (sv *Service) applyEntry(ent raftpb.Entry) error {
	log.Write(context.Background(), "index", ent.Index, "ent", ent)

	switch ent.Type {
	case raftpb.EntryConfChange:
		var cc raftpb.ConfChange
		err := cc.Unmarshal(ent.Data)
		if err != nil {
			return errors.Wrap(err)
		}
		sv.raftNode.ApplyConfChange(cc)
		switch cc.Type {
		case raftpb.ConfChangeAddNode:
			log.Write(context.Background(),
				"what", "adding node",
				"id", cc.NodeID,
				"addr", string(cc.Context),
			)
			sv.stateMu.Lock()
			sv.peers[cc.NodeID] = string(cc.Context)
			sv.stateMu.Unlock()
		case raftpb.ConfChangeRemoveNode:
			if cc.NodeID == sv.id {
				// log.Println("I've been removed from the cluster! Shutting down.")
				// TODO(kr): stop doing raft somehow?
				// or attempt to rejoin as a new follower???
				panic("evicted")
			}
			// TODO(kr): sv.removePeer(cc.NodeID)
		}
	case raftpb.EntryNormal:
		if ent.Index < sv.appliedIndex {
			return nil
		}
		log.Write(context.Background(), "EntryNormal", ent)
		if len(ent.Data) == 0 {
			return nil
		}
		switch ent.Data[0] {
		case propWrite:
			var appEntry map[string]string
			err := json.Unmarshal(ent.Data[1:], &appEntry)
			if err != nil {
				// An error here indicates a malformed update
				// was written to the raft log. We do version
				// negotiation in the transport layer, so this
				// should be impossible; by this point, we are
				// all speaking the same version.
				return errors.Wrap(err)
			}
			sv.stateMu.Lock()
			for k, v := range appEntry {
				sv.state[k] = v
			}
			sv.stateMu.Unlock()
		case propRead:
			// nothing
		}
	default:
		return errors.Wrap(fmt.Errorf("unknown entry type: %v", ent))
	}

	sv.appliedIndex = ent.Index
	return nil
}

func (sv *Service) send(msgs []raftpb.Message) error {
	for _, msg := range msgs {
		data, err := msg.Marshal()
		if err != nil {
			return errors.Wrap(err)
		}
		sv.stateMu.Lock()
		addr, ok := sv.peers[msg.To]
		sv.stateMu.Unlock()
		if !ok {
			log.Write(context.Background(), "no-addr-for-peer", msg.To)
			continue
		}
		sendmsg(addr, data)
	}
	return nil
}

// best effort. if it fails, oh well -- that's why we're using raft.
func sendmsg(addr string, data []byte) {
	resp, err := http.Post("http://"+addr+"/raft/msg", contentType, bytes.NewReader(data))
	if err != nil {
		log.Error(context.Background(), err)
		return
	}
	defer resp.Body.Close()
}

// recover loads state from the last full snapshot,
// then replays WAL entries into the raft instance.
// The returned WAL object is nil if no WAL is found.
func (sv *Service) recover() (*wal.WAL, error) {
	log.Write(context.Background(), "waldir", sv.walDir())
	err := os.MkdirAll(sv.walDir(), 0777)
	if err != nil {
		return nil, errors.Wrap(err)
	}

	var raftSnap raftpb.Snapshot
	snapData, err := ioutil.ReadFile(sv.snapFile())
	if err != nil && !os.IsNotExist(err) {
		return nil, errors.Wrap(err)
	}
	if err == nil {
		err = decodeSnapshot(snapData, &raftSnap)
		if err != nil {
			return nil, errors.Wrap(err)
		}
	}

	if ents, err := ioutil.ReadDir(sv.walDir()); err == nil && len(ents) == 0 {
		return nil, nil
	}

	wal, err := wal.Open(sv.walDir(), walpb.Snapshot{
		Index: raftSnap.Metadata.Index,
		Term:  raftSnap.Metadata.Term,
	})
	if err != nil {
		return nil, errors.Wrap(err)
	}

	_, st, ents, err := wal.ReadAll()
	if err != nil {
		return nil, errors.Wrap(err)
	}

	sv.raftStorage.ApplySnapshot(raftSnap)
	if raft.IsEmptySnap(raftSnap) {
		sv.state = make(map[string]string)
	} else {
		var snap snapshot
		// TODO(kr): figure out a better snapshot encoding
		err = json.Unmarshal(raftSnap.Data, &snap)
		if err != nil {
			return nil, errors.Wrap(err)
		}
		sv.state = snap.State
		sv.peers = snap.Peers
		sv.confState = raftSnap.Metadata.ConfState
		sv.snapIndex = raftSnap.Metadata.Index
		sv.appliedIndex = raftSnap.Metadata.Index
	}

	sv.raftStorage.SetHardState(st)
	sv.raftStorage.Append(ents)
	for _, ent := range ents {
		err = sv.applyEntry(ent)
		if err != nil {
			return nil, err
		}
	}

	return wal, nil
}

func (sv *Service) getSnapshot() (*raftpb.Snapshot, error) {
	// TODO(kr): figure out a better snapshot encoding
	sv.stateMu.Lock()
	v := snapshot{sv.state, sv.peers}
	sv.stateMu.Unlock()
	ctx := context.Background()
	log.Messagef(ctx, "encoding snapshot %#v", v)
	data, err := json.Marshal(v)
	if err != nil {
		return nil, errors.Wrap(err)
	}
	snap, err := sv.raftStorage.CreateSnapshot(sv.appliedIndex, &sv.confState, data)
	return &snap, errors.Wrap(err)
}

func (sv *Service) triggerSnapshot() error {
	snap, err := sv.getSnapshot()
	if err != nil {
		return errors.Wrap(err)
	}
	err = sv.saveSnapshot(snap)
	if err != nil {
		return errors.Wrap(err)
	}

	var compactIndex uint64 = 1
	if sv.appliedIndex > nSnapCatchupEntries {
		compactIndex = sv.appliedIndex - nSnapCatchupEntries
	}
	err = sv.raftStorage.Compact(compactIndex)
	if err != nil {
		return errors.Wrap(err)
	}
	sv.snapIndex = sv.appliedIndex
	return nil
}

func (sv *Service) saveSnapshot(snapshot *raftpb.Snapshot) error {
	d, err := encodeSnapshot(snapshot)
	if err != nil {
		return errors.Wrap(err)
	}
	return writeFile(sv.snapFile(), d, 0666)
}

func readID(dir string) (uint64, error) {
	d, err := ioutil.ReadFile(filepath.Join(dir, "id"))
	if err != nil {
		return 0, err
	}
	if len(d) != 12 {
		return 0, errors.New("bad id file size")
	}
	if crc32.Checksum(d[:8], crcTable) != binary.BigEndian.Uint32(d[8:]) {
		return 0, fmt.Errorf("bad CRC in member id %x", d)
	}
	return binary.BigEndian.Uint64(d), nil
}

func writeID(dir string, id uint64) error {
	b := make([]byte, 12)
	binary.BigEndian.PutUint64(b, id)
	binary.BigEndian.PutUint32(b[8:], crc32.Checksum(b[:8], crcTable))
	name := filepath.Join(dir, "id")
	return errors.Wrap(writeFile(name, b, 0666))
}

func (sv *Service) walDir() string   { return filepath.Join(sv.dir, "wal") }
func (sv *Service) snapFile() string { return filepath.Join(sv.dir, "snap") }

// redo runs f repeatedly until it returns nil, with exponential backoff.
// It reports any errors encountered using sv.Error.
// It must be called nowhere but runUpdates.
func (sv *Service) redo(f func() error) {
	var n uint
	for {
		err := f()
		if err != nil {
			sv.errMu.Lock()
			sv.err = err
			sv.errMu.Unlock()
			n++
			time.Sleep(100*time.Millisecond + time.Millisecond<<n)
		} else {
			sv.errMu.Lock()
			sv.err = nil
			sv.errMu.Unlock()
			break
		}
	}
}

type staleGetter Service

func (g *staleGetter) Get(key string) string {
	g.stateMu.Lock()
	defer g.stateMu.Unlock()
	return g.state[key]
}
