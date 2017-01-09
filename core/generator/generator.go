// Package generator implements the Chain Core generator.
//
// A Chain Core configured as a generator produces new blocks
// on an interval.
package generator

import (
	"context"
	"time"

	"chain/database/pg"
	"chain/log"
	"chain/protocol"
	"chain/protocol/bc"
	"chain/protocol/state"
	"chain/protocol/validation"
)

// A BlockSigner signs blocks.
type BlockSigner interface {
	// SignBlock returns an ed25519 signature over the block's sighash.
	// See also the Chain Protocol spec for the complete required behavior
	// of a block signer.
	SignBlock(context.Context, *bc.Block) (signature []byte, err error)
}

// Generator collects pending transactions and produces new blocks on
// an interval.
type Generator struct {
	// config
	db      pg.DB
	chain   *protocol.Chain
	signers []BlockSigner

	pool        []*bc.Tx // in topological order
	poolHashes  map[bc.Hash]bool
	incomingTxs chan *bc.Tx

	pendingBlock    *bc.Block
	pendingSnapshot *state.Snapshot

	// latestBlock and latestSnapshot are current as long as this
	// process remains the leader process. If the process is demoted,
	// generator.Generate() should return and this struct should be
	// garbage collected.
	latestBlock    *bc.Block
	latestSnapshot *state.Snapshot
}

// New creates and initializes a new Generator.
func New(
	c *protocol.Chain,
	s []BlockSigner,
	db pg.DB,
) *Generator {
	return &Generator{
		db:          db,
		chain:       c,
		signers:     s,
		poolHashes:  make(map[bc.Hash]bool),
		incomingTxs: make(chan *bc.Tx),
	}
}

// Submit adds a new pending tx to the pending tx pool.
func (g *Generator) Submit(ctx context.Context, tx *bc.Tx) error {
	g.incomingTxs <- tx
	return nil
}

// Generate runs in a loop, making one new block
// every block period. It returns when its context
// is canceled.
// After each attempt to make a block, it calls health
// to report either an error or nil to indicate success.
func (g *Generator) Generate(
	ctx context.Context,
	period time.Duration,
	health func(error),
) {
	// This process just became leader, so it's responsible
	// for recovering after the previous leader's exit.
	recoveredBlock, recoveredSnapshot, err := g.chain.Recover(ctx)
	if err != nil {
		log.Fatal(ctx, log.KeyError, err)
	}
	g.latestBlock, g.latestSnapshot = recoveredBlock, recoveredSnapshot
	g.pendingSnapshot = state.Copy(recoveredSnapshot)

	// Check to see if we already have a pending, generated block.
	// This can happen if the leader process exits between generating
	// the block and committing the signed block to the blockchain.
	b, err := getPendingBlock(ctx, g.db)
	if err != nil {
		log.Fatal(ctx, log.KeyError, err)
	}
	if b != nil && (g.latestBlock == nil || b.Height == g.latestBlock.Height+1) {
		s := state.Copy(g.latestSnapshot)
		err := validation.ApplyBlock(s, b)
		if err != nil {
			log.Fatal(ctx, log.KeyError, err)
		}

		err = g.commitBlock(ctx, b, s)
		if err != nil {
			log.Fatal(ctx, log.KeyError, err)
		}
		g.latestBlock, g.latestSnapshot = b, s
		g.pendingSnapshot = state.Copy(s)
	}

	lastTick := time.Now()
	ticks := time.Tick(period)
	for {
		select {
		case <-ctx.Done():
			log.Messagef(ctx, "Deposed, Generate exiting")
			return
		case t := <-ticks:
			lastTick = t
			if g.pendingBlock == nil {
				// No transactions have been received, so no block was
				// created.
				continue
			}

			// Finalize the pending block.
			b := g.pendingBlock
			b.TransactionsMerkleRoot = validation.CalcMerkleRoot(b.Transactions)
			b.AssetsMerkleRoot = g.pendingSnapshot.Tree.RootHash()

			err = savePendingBlock(ctx, g.db, b)
			if err != nil {
				health(err)
				log.Error(ctx, err)
				continue
			}

			err = g.commitBlock(ctx, b, g.pendingSnapshot)
			if err != nil {
				health(err)
				log.Error(ctx, err)
				// Give up on the pending block and start over.
				g.pendingBlock = nil
				g.pendingSnapshot = state.Copy(g.latestSnapshot)
				continue
			}

			// We committed a new block. Update the pending and current state pointers.
			health(nil)
			g.latestBlock = b
			g.latestSnapshot = g.pendingSnapshot
			g.pendingBlock = nil
			g.pendingSnapshot = state.Copy(g.latestSnapshot)

		case tx := <-g.incomingTxs:
			if g.poolHashes[tx.Hash] {
				continue
			}
			if g.pendingBlock != nil && len(g.pendingBlock.Transactions) >= 10000 {
				log.Messagef(ctx, "Dropping tx %s, current block already full", tx.Hash)
				continue
			}

			// Create a new pending block if there's not one already.
			if g.pendingBlock == nil {
				g.pendingBlock = &bc.Block{
					BlockHeader: bc.BlockHeader{
						Version:           bc.NewBlockVersion,
						Height:            g.latestBlock.Height + 1,
						PreviousBlockHash: g.latestBlock.Hash(),
						TimestampMS:       bc.Millis(lastTick.Add(period)),
						ConsensusProgram:  g.latestBlock.ConsensusProgram,
					},
				}
				g.pendingSnapshot.PruneIssuances(g.pendingBlock.TimestampMS)
			}

			err = g.chain.CheckIssuanceWindow(tx)
			if err != nil {
				continue // rejected
			}
			err = validation.ConfirmTx(g.pendingSnapshot, g.chain.InitialBlockHash, g.pendingBlock, tx)
			if err != nil {
				continue // rejected
			}
			err = validation.ApplyTx(g.pendingSnapshot, tx)
			if err != nil {
				continue // rejected
			}
			g.pendingBlock.Transactions = append(g.pendingBlock.Transactions, tx)
			g.poolHashes[tx.Hash] = true
		}
	}
}
