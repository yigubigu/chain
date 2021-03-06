package protocol

import (
	"context"
	"log"
	"testing"
	"time"

	"chain/protocol/bc"
	"chain/protocol/memstore"
	"chain/protocol/state"
	"chain/protocol/validation"
	"chain/testutil"
)

func TestRecoverSnapshotNoAdditionalBlocks(t *testing.T) {
	store := memstore.New()
	b, err := NewInitialBlock(nil, 0, time.Now())
	if err != nil {
		testutil.FatalErr(t, err)
	}
	c1, err := NewChain(context.Background(), b.Hash(), store, nil)
	if err != nil {
		t.Fatal(err)
	}
	err = c1.CommitBlock(context.Background(), b, state.Empty())
	if err != nil {
		testutil.FatalErr(t, err)
	}

	// Snapshots are applied asynchronously. This loops waits
	// until the snapshot is created.
	for {
		_, height, _ := store.LatestSnapshot(context.Background())
		if height > 0 {
			break
		}
	}

	c2, err := NewChain(context.Background(), b.Hash(), store, nil)
	if err != nil {
		t.Fatal(err)
	}
	block, snapshot, err := c2.Recover(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if block.Height != 1 {
		t.Fatalf("block.Height = %d, want %d", block.Height, 1)
	}

	err = c2.ValidateBlockForSig(context.Background(), createEmptyBlock(block, snapshot))
	if err != nil {
		t.Fatal(err)
	}
}

func createEmptyBlock(block *bc.Block, snapshot *state.Snapshot) *bc.Block {
	root, err := validation.CalcMerkleRoot(nil)
	if err != nil {
		log.Fatalf("calculating empty merkle root: %s", err)
	}

	return &bc.Block{
		BlockHeader: bc.BlockHeader{
			Version:           bc.NewBlockVersion,
			Height:            block.Height + 1,
			PreviousBlockHash: block.Hash(),
			TimestampMS:       bc.Millis(time.Now()),
			BlockCommitment: bc.BlockCommitment{
				TransactionsMerkleRoot: root,
				AssetsMerkleRoot:       snapshot.Tree.RootHash(),
				ConsensusProgram:       block.ConsensusProgram,
			},
		},
	}
}
