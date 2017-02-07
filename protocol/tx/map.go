package tx

import (
	"fmt"

	"chain/errors"
	"chain/protocol/bc"
	"chain/protocol/vm"
	"chain/protocol/vmutil"
)

func mapTx(tx *bc.TxData) (headerID bc.Hash, hdr *Header, entryMap map[bc.Hash]Entry, err error) {
	var dataRef bc.Hash

	entryMap = make(map[bc.Hash]Entry)

	addEntry := func(e Entry) (id bc.Hash, entry Entry, err error) {
		id, err = entryID(e)
		if err != nil {
			err = errors.Wrapf(err, "computing entryID for %s entry", e.Type())
			return
		}
		entryMap[id] = e
		return id, e, nil
	}

	if len(tx.ReferenceData) > 0 {
		dataRef, _, err = addEntry(newData(hashData(tx.ReferenceData)))
		if err != nil {
			err = errors.Wrap(err, "adding refdata entry")
			return
		}
	}

	// Loop twice over tx.Inputs, once for spends and once for
	// issuances.  Do spends first so the entry ID of the first spend is
	// available in case an issuance needs it for its anchor.

	var firstSpendID *bc.Hash
	muxSources := make([]valueSource, len(tx.Inputs))

	for i, inp := range tx.Inputs {
		if oldSp, ok := inp.TypedInput.(*bc.SpendInput); ok {
			var inpDataRef bc.Hash
			if len(inp.ReferenceData) != 0 {
				inpDataRef, _, err = addEntry(newData(hashData(inp.ReferenceData)))
				if err != nil {
					return
				}
			}
			var spID bc.Hash
			spID, _, err = addEntry(newSpend(oldSp.SpentOutputID, inpDataRef, i))
			if err != nil {
				err = errors.Wrapf(err, "adding spend entry for input %d", i)
				return
			}
			muxSources[i] = valueSource{
				Ref:   spID,
				Value: oldSp.AssetAmount,
			}
			if firstSpendID == nil {
				firstSpendID = &spID
			}
		}
	}

	for i, inp := range tx.Inputs {
		if oldIss, ok := inp.TypedInput.(*bc.IssuanceInput); ok {
			var inpDataRef bc.Hash
			if len(inp.ReferenceData) != 0 {
				inpDataRef, _, err = addEntry(newData(hashData(inp.ReferenceData)))
				if err != nil {
					err = errors.Wrapf(err, "adding input refdata entry for input %d", i)
					return
				}
			}

			// Note: asset definitions, initial block ids, and issuance
			// programs are omitted here because they do not contribute to
			// the body hash of an issuance.

			var nonceRef bc.Hash

			if len(oldIss.Nonce) == 0 {
				if firstSpendID == nil {
					err = fmt.Errorf("nonce-less issuance in transaction with no spends")
					return
				}
				nonceRef = *firstSpendID
			} else {
				var trID bc.Hash
				trID, _, err = addEntry(newTimeRange(tx.MinTime, tx.MaxTime))
				if err != nil {
					err = errors.Wrapf(err, "adding timerange entry for input %d", i)
					return
				}

				assetID := oldIss.AssetID()
				b := vmutil.NewBuilder()
				b = b.AddData(oldIss.Nonce).AddOp(vm.OP_DROP).AddOp(vm.OP_ASSET).AddData(assetID[:]).AddOp(vm.OP_EQUAL)

				nonceRef, _, err = addEntry(newNonce(bc.Program{1, b.Program}, trID))
				if err != nil {
					err = errors.Wrapf(err, "adding nonce entry for input %d", i)
					return
				}
			}

			val := inp.AssetAmount()

			var issID bc.Hash
			issID, _, err = addEntry(newIssuance(nonceRef, val, inpDataRef, i))
			if err != nil {
				err = errors.Wrapf(err, "adding issuance entry for input %d", i)
				return
			}

			muxSources[i] = valueSource{
				Ref:   issID,
				Value: val,
			}
		}
	}

	muxID, _, err := addEntry(newMux(muxSources))
	if err != nil {
		err = errors.Wrap(err, "adding mux entry")
		return
	}

	var resultRefs []bc.Hash

	for i, out := range tx.Outputs {
		s := valueSource{
			Ref:      muxID,
			Position: uint64(i),
			Value:    out.AssetAmount,
		}

		var outDataRef bc.Hash
		if len(out.ReferenceData) > 0 {
			outDataRef, _, err = addEntry(newData(hashData(out.ReferenceData)))
			if err != nil {
				err = errors.Wrapf(err, "adding refdata entry for output %d", i)
				return
			}
		}

		var resultID bc.Hash
		if vmutil.IsUnspendable(out.ControlProgram) {
			// retirement
			resultID, _, err = addEntry(newRetirement(s, outDataRef, i))
			if err != nil {
				err = errors.Wrapf(err, "adding retirement entry for output %d", i)
				return
			}
		} else {
			// non-retirement
			prog := bc.Program{out.VMVersion, out.ControlProgram}
			resultID, _, err = addEntry(newOutput(s, prog, outDataRef, i))
			if err != nil {
				err = errors.Wrapf(err, "adding output entry for output %d", i)
				return
			}
		}

		resultRefs = append(resultRefs, resultID)
	}

	var h Entry
	headerID, h, err = addEntry(newHeader(tx.Version, resultRefs, dataRef, tx.MinTime, tx.MaxTime))
	if err != nil {
		err = errors.Wrap(err, "adding header entry")
		return
	}

	return headerID, h.(*Header), entryMap, nil
}

func mapBlockHeader(old *bc.BlockHeader) (bhID entryRef, bh *blockHeader, err error) {
	bh = newBlockHeader(old.Version, old.Height, entryRef(old.PreviousBlockHash), old.TimestampMS, old.TransactionsMerkleRoot, old.AssetsMerkleRoot, old.ConsensusProgram)
	bhID, err = entryID(bh)
	return
}
