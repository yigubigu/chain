package tx

import (
	"fmt"

	"chain/crypto/sha3pool"
	"chain/errors"
	"chain/protocol/bc"
)

func init() {
	bc.TxHashesFunc = TxHashes
	bc.BlockHeaderHashFunc = func(old *bc.BlockHeader) (bc.Hash, error) {
		hash, _, err := mapBlockHeader(old)
		return bc.Hash(hash), err
	}
}

// TxHashes returns all hashes needed for validation and state updates.
func TxHashes(oldTx *bc.TxData) (hashes *bc.TxHashes, err error) {
	txid, header, entries, err := mapTx(oldTx)
	if err != nil {
		return nil, errors.Wrap(err, "mapping old transaction to new")
	}

	hashes = new(bc.TxHashes)
	hashes.ID = bc.Hash(txid)

	// OutputIDs
	hashes.OutputIDs = make([]bc.Hash, len(header.body.ResultRefs))
	for i, resultHash := range header.body.ResultRefs {
		result := entries[resultHash]
		if _, ok := result.(*Output); ok {
			hashes.OutputIDs[i] = bc.Hash(resultHash)
		}
	}

	var txRefDataHash bc.Hash
	if header.body.DataRef == (bc.Hash{}) {
		// no data entry
		txRefDataHash = bc.EmptyStringHash
	} else {
		dEntry, ok := entries[header.body.DataRef]
		if !ok {
			return nil, fmt.Errorf("header refers to nonexistent data entry")
		}
		d, ok := dEntry.(*data)
		if !ok {
			return nil, fmt.Errorf("header refers to %s entry, should be data", dEntry.Type())
		}
		txRefDataHash = d.body
	}

	hashes.VMContexts = make([]*bc.VMContext, len(oldTx.Inputs))

	getRefDataHash := func(id bc.Hash) (bc.Hash, error) {
		dEntry, ok := entries[id]
		if !ok {
			return bc.EmptyStringHash, nil
		}
		d, ok := dEntry.(*data)
		if !ok {
			return bc.Hash{}, fmt.Errorf("unexpected type %T for entry %x", dEntry, id[:])
		}
		return d.body, nil
	}

	for entryID, ent := range entries {
		switch ent := ent.(type) {
		case *Nonce:
			// TODO: check time range is within network-defined limits
			trID := ent.body.TimeRangeRef
			trEntry, ok := entries[trID]
			if !ok {
				return nil, fmt.Errorf("nonce entry refers to nonexistent timerange entry")
			}
			tr, ok := trEntry.(*TimeRange)
			if !ok {
				return nil, fmt.Errorf("nonce entry refers to %s entry, should be timerange", trEntry.Type())
			}
			iss := struct {
				ID           bc.Hash
				ExpirationMS uint64
			}{entryID, tr.body.MaxTimeMS}
			hashes.Issuances = append(hashes.Issuances, iss)

		case *Issuance:
			vmc := newVMContext(entryID, hashes.ID, txRefDataHash)
			vmc.RefDataHash, err = getRefDataHash(ent.body.DataRef)
			if err != nil {
				return nil, err
			}
			vmc.NonceID = &ent.body.AnchorRef
			hashes.VMContexts[ent.Ordinal()] = vmc

		case *Spend:
			vmc := newVMContext(entryID, hashes.ID, txRefDataHash)
			vmc.RefDataHash, err = getRefDataHash(ent.body.DataRef)
			if err != nil {
				return nil, err
			}
			vmc.OutputID = &ent.body.SpentOutput
			hashes.VMContexts[ent.Ordinal()] = vmc
		}
	}

	return hashes, nil
}

// populates the common fields of a VMContext for an Entry, regardless of whether
// that Entry is a Spend or an Issuance
func newVMContext(entryID, txid, txRefDataHash bc.Hash) *bc.VMContext {
	vmc := new(bc.VMContext)

	// TxRefDataHash
	vmc.TxRefDataHash = txRefDataHash

	// EntryID
	vmc.EntryID = entryID

	// TxSigHash
	hasher := sha3pool.Get256()
	defer sha3pool.Put256(hasher)
	hasher.Write(entryID[:])
	hasher.Write(txid[:])
	hasher.Read(vmc.TxSigHash[:])

	return vmc
}
