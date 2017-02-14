package validation

import (
	"math"

	"golang.org/x/sync/errgroup"

	"chain/errors"
	"chain/math/checked"
	"chain/protocol/bc"
	"chain/protocol/state"
	"chain/protocol/vm"
	"chain/protocol/vmutil"
)

// ErrBadTx is returned for transactions failing validation
var ErrBadTx = errors.New("invalid transaction")

var (
	// "suberrors" for ErrBadTx
	errTxVersion              = errors.New("unknown transaction version")
	errNotYet                 = errors.New("block time is before transaction min time")
	errTooLate                = errors.New("block time is after transaction max time")
	errWrongBlockchain        = errors.New("issuance is for different blockchain")
	errTimelessIssuance       = errors.New("zero mintime or maxtime not allowed in issuance with non-empty nonce")
	errIssuanceTime           = errors.New("timestamp outside issuance input's time window")
	errDuplicateIssuance      = errors.New("duplicate issuance transaction")
	errInvalidOutput          = errors.New("invalid output")
	errNoInputs               = errors.New("inputs are missing")
	errTooManyInputs          = errors.New("number of inputs overflows uint32")
	errAllEmptyNonceIssuances = errors.New("all inputs are issuances with empty nonce fields")
	errMisorderedTime         = errors.New("positive maxtime must be >= mintime")
	errAssetVersion           = errors.New("unknown asset version")
	errInputTooBig            = errors.New("input value exceeds maximum value of int64")
	errInputSumTooBig         = errors.New("sum of inputs overflows the allowed asset amount")
	errVMVersion              = errors.New("unknown vm version")
	errDuplicateInput         = errors.New("duplicate input")
	errTooManyOutputs         = errors.New("number of outputs overflows int32")
	errEmptyOutput            = errors.New("output value must be greater than 0")
	errOutputTooBig           = errors.New("output value exceeds maximum value of int64")
	errOutputSumTooBig        = errors.New("sum of outputs overflows the allowed asset amount")
	errUnbalancedV1           = errors.New("amounts for asset are not balanced on v1 inputs and outputs")
)

func badTxErr(err error) error {
	err = errors.WithData(err, "badtx", err)
	err = errors.WithDetail(err, err.Error())
	return errors.Sub(ErrBadTx, err)
}

func badTxErrf(err error, f string, args ...interface{}) error {
	err = errors.WithData(err, "badtx", err)
	err = errors.WithDetailf(err, f, args...)
	return errors.Sub(ErrBadTx, err)
}

// ConfirmTx validates the given transaction against the given state tree
// before it's added to a block. If tx is invalid, it returns a non-nil
// error describing why.
//
// Tx must already have undergone the well-formedness check in
// CheckTxWellFormed. This should have happened when the tx was added
// to the pool.
//
// ConfirmTx must not mutate the snapshot or the block.
func ConfirmTx(snapshot *state.Snapshot, initialBlockHash bc.Hash, block *bc.Block, tx *bc.Tx) error {
	if tx.Version < 1 || tx.Version > block.Version {
		return badTxErrf(errTxVersion, "unknown transaction version %d for block version %d", tx.Version, block.Version)
	}

	if block.TimestampMS < tx.MinTime {
		return badTxErr(errNotYet)
	}
	if tx.MaxTime > 0 && block.TimestampMS > tx.MaxTime {
		return badTxErr(errTooLate)
	}

	for i, txin := range tx.Inputs {
		if ii, ok := txin.TypedInput.(*bc.IssuanceInput); ok {
			if txin.AssetVersion != 1 {
				continue
			}
			if ii.InitialBlock != initialBlockHash {
				return badTxErr(errWrongBlockchain)
			}
			if len(ii.Nonce) == 0 {
				continue
			}
			if tx.MinTime == 0 || tx.MaxTime == 0 {
				return badTxErr(errTimelessIssuance)
			}
			if block.TimestampMS < tx.MinTime || block.TimestampMS > tx.MaxTime {
				return badTxErr(errIssuanceTime)
			}
			iHash, err := tx.IssuanceHash(i)
			if err != nil {
				return err
			}
			if _, ok2 := snapshot.Issuances[iHash]; ok2 {
				return badTxErr(errDuplicateIssuance)
			}
			continue
		}

		// txin is a spend

		// Lookup the prevout in the blockchain state tree.
		k, val := state.OutputTreeItem(state.Prevout(txin))
		if !snapshot.Tree.Contains(k, val) {
			return badTxErrf(errInvalidOutput, "output %s for input %d is invalid", txin.SpentOutputID().String(), i)
		}
	}
	return nil
}

// CheckTxWellFormed checks whether tx is "well-formed" (the
// context-free phase of validation):
// - inputs and outputs balance
// - no duplicate input commitments
// - input scripts pass
//
// Result is nil for well-formed transactions, ErrBadTx with
// supporting detail otherwise.
func CheckTxWellFormed(hdrRef *bc.EntryRef) error {
	hdr := hdrRef.Entry.(*tx.Header)
	spends, issuances, err := hdr.Inputs()
	if err != nil {
		return err
	}

	nInputs := len(spends) + len(issuances)
	if nInputs == 0 {
		return badTxErr(errNoInputs)
	}
	if nInputs > math.MaxInt32 {
		return badTxErr(errTooManyInputs)
	}

	// Are all inputs issuances, all with asset version 1, and all with empty nonces?
	allIssuancesWithEmptyNonces := true
	if len(spends) > 0 {
		allIssuancesWithEmptyNonces = false
	} else {
		for _, issRef := range issuances {
			iss, ok := issRef.Entry.(*tx.Issuance)
			if !ok {
				// xxx (impossible?) error
			}
			if !iss.Anchor().IsNil() { // xxx is this the right test?
				allIssuancesWithEmptyNonces = false
				break
			}
		}
	}
	if allIssuancesWithEmptyNonces {
		return badTxErr(errAllEmptyNonceIssuances)
	}

	// Check that the transaction maximum time is greater than or equal to the
	// minimum time, if it is greater than 0.
	if hdr.MaxTimeMS() > 0 && hdr.MaxTimeMS() < hdr.MinTimeMS() {
		return badTxErr(errMisorderedTime)
	}

	txVersion := hdr.Version()

	// Check that each input commitment appears only once. Also check that sums
	// of inputs and outputs balance, and check that both input and output sums
	// are less than 2^63 so that they don't overflow their int64 representation.
	parity := make(map[bc.AssetID]int64)

	for _, spRef := range spends {
		sp := spRef.Entry.(*tx.Spend)
		outRef := sp.SpentOutput()
		out := outRef.Entry.(*tx.Output)
		amount := out.Amount()
		if amount > math.MaxInt64 {
			return badTxErr(errInputTooBig)
		}
		assetID := out.AssetID()
		sum, ok := checked.AddInt64(parity[assetID], int64(amount))
		if !ok {
			id, _ := spRef.Hash()
			return badTxErrf(errInputSumTooBig, "adding input %x overflows the allowed asset amount", id[:])
		}
		parity[assetID] = sum
		if txVersion == 1 {
			prog := out.ControlProgram()
			if prog.VMVersion != 1 {
				id, _ := spRef.Hash()
				outID, _ := outRef.Hash()
				return badTxErrf(errVMVersion, "unknown vm version %d in input %x (spending output %x) for transaction version %d", prog.VMVersion, id[:], outID[:], txVersion)
			}
		}
	}

	for _, issRef := range issuances {
		iss := issRef.Entry.(*tx.Issuance)
		amount := iss.Amount()
		if amount > math.MaxInt64 {
			return badTxErr(errInputTooBig)
		}
		assetID := iss.AssetID()
		sum, ok := checked.AddInt64(parity[assetID], int64(amount))
		if !ok {
			id, _ := issRef.Hash()
			return badTxErrf(errInputSumTooBig, "adding input %x overflows the allowed asset amount", id[:])
		}
		parity[assetID] = sum
		if txVersion == 1 {
			prog := iss.IssuanceProgram()
			if prog.VMVersion != 1 {
				id, _ := issRef.Hash()
				return badTxErrf(errVMVersion, "unknown vm version %d in input %x for transaction version %d", prog.VMVersion, id[:], txVersion)
			}
		}
		nonceRef := iss.Anchor()
		_, ok = nonceRef.Entry.(*tx.Nonce)
		if !ok {
			// xxx
		}
	}

	outRefs := hdr.Results()
	if len(outRefs) > math.MaxInt32 {
		return badTxErr(errTooManyOutputs)
	}

	for _, outRef := range outRefs {
		out := outRef.Entry.(*tx.Output)
		if txVersion == 1 {
			prog := out.ControlProgram()
			if prog.VMVersion != 1 {
				id, _ := outRef.Hash()
				return badTxErrf(errVMVersion, "unknown vm version %d in output %x for transaction version %d", prog.VMVersion, id[:], txVersion)
			}
		}
		amount := out.Amount()
		if amount == 0 {
			return badTxErr(errEmptyOutput)
		}
		if amount > math.MaxInt64 {
			return badTxErr(errOutputTooBig)
		}
		assetID := out.AssetID()
		sum, ok := checked.SubInt64(parity[assetID], int64(amount))
		if !ok {
			id, _ := outRef.Hash()
			return badTxErrf(errOutputSumTooBig, "adding output %x (%d units of asset %x) overflows the allowed asset amount", id[:], amount, assetID[:])
		}
		parity[assetID] = sum
	}

	for assetID, val := range parity {
		if val != 0 {
			return badTxErrf(errUnbalancedV1, "amounts for asset %s are not balanced on v1 inputs and outputs", assetID)
		}
	}

	// verifyFn returns a closure suitable for use in errgroup.Group.Go
	verifyFn := func(e *bc.EntryRef) func() error {
		return func() error {
			err := vm.VerifyTxInput(hdrRef, e)
			if err != nil {
				id, _ := e.Hash()
				return badTxErrf(err, "validation failed in script execution, input %x", id[:])
			}
			return nil
		}
	}

	var g errgroup.Group
	for _, spRef := range spends {
		g.Go(verifyFn(spRef))
	}
	for _, issRef := range issuances {
		g.Go(verifyFn(issRef))
	}

	return g.Wait()
}

// ApplyTx updates the state tree with all the changes to the ledger.
func ApplyTx(snapshot *state.Snapshot, tx *bc.Tx) error {
	for i, in := range tx.Inputs {
		if ii, ok := in.TypedInput.(*bc.IssuanceInput); ok {
			if len(ii.Nonce) > 0 {
				iHash, err := tx.IssuanceHash(i)
				if err != nil {
					return err
				}
				snapshot.Issuances[iHash] = tx.MaxTime
			}
			continue
		}

		si := in.TypedInput.(*bc.SpendInput)

		// Remove the consumed output from the state tree.
		uid := si.SpentOutputID
		err := snapshot.Tree.Delete(uid.Bytes())
		if err != nil {
			return err
		}
	}

	for i, out := range tx.Outputs {
		if vmutil.IsUnspendable(out.ControlProgram) {
			continue
		}
		// Insert new outputs into the state tree.
		o := state.NewOutput(*out, tx.OutputID(uint32(i)))

		err := snapshot.Tree.Insert(state.OutputTreeItem(o))
		if err != nil {
			return err
		}
	}
	return nil
}
