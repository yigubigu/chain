package vm

import (
	"bytes"
	"fmt"
	"math"

	"chain/protocol/bc"
	"chain/protocol/tx"
)

func opCheckOutput(vm *virtualMachine) error {
	if vm.txHeaderRef.IsNil() {
		return ErrContext
	}

	err := vm.applyCost(16)
	if err != nil {
		return err
	}

	code, err := vm.pop(true)
	if err != nil {
		return err
	}
	vmVersion, err := vm.popInt64(true)
	if err != nil {
		return err
	}
	if vmVersion < 0 {
		return ErrBadValue
	}
	assetID, err := vm.pop(true)
	if err != nil {
		return err
	}
	amount, err := vm.popInt64(true)
	if err != nil {
		return err
	}
	if amount < 0 {
		return ErrBadValue
	}
	refdatahash, err := vm.pop(true)
	if err != nil {
		return err
	}
	outputID, err := vm.pop(true)
	if err != nil {
		return err
	}
	// xxx check len(outputID)

	var o *tx.Output
	hdr := vm.txHeaderRef.Entry.(*tx.Header)
	for _, resultRef := range hdr.Results() {
		id, err := resultRef.Hash()
		if err != nil {
			// xxx
		}
		if bytes.Equal(outputID, id[:]) {
			o = resultRef.Entry.(*tx.Output)
			break
		}
	}
	if o == nil {
		return ErrBadValue
	}

	if o.Amount() != uint64(amount) {
		return vm.pushBool(false, true)
	}
	prog := o.ControlProgram()
	if prog.VMVersion != uint64(vmVersion) {
		return vm.pushBool(false, true)
	}
	if !bytes.Equal(prog.Code, code) {
		return vm.pushBool(false, true)
	}
	oAssetID := o.AssetID()
	if !bytes.Equal(oAssetID[:], assetID) {
		return vm.pushBool(false, true)
	}
	if len(refdatahash) > 0 {
		oRefDataHash := o.RefDataHash()
		if !bytes.Equal(oRefDataHash[:], refdatahash) {
			return vm.pushBool(false, true)
		}
	}
	return vm.pushBool(true, true)
}

func opAsset(vm *virtualMachine) error {
	if vm.txHeaderRef.IsNil() {
		return ErrContext
	}

	err := vm.applyCost(1)
	if err != nil {
		return err
	}

	var assetID bc.AssetID

	switch e := vm.input.Entry.(type) {
	case *tx.Spend:
		oEntry := e.SpentOutput().Entry
		if oEntry == nil {
			// xxx error
		}
		o, ok := oEntry.(*tx.Output)
		if !ok {
			// xxx error
		}
		assetID = o.AssetID()

	case *tx.Issuance:
		assetID = e.AssetID()

	default:
		// xxx error
	}

	return vm.push(assetID[:], true)
}

func opAmount(vm *virtualMachine) error {
	if vm.txHeaderRef.IsNil() {
		return ErrContext
	}

	err := vm.applyCost(1)
	if err != nil {
		return err
	}

	var amount uint64

	switch e := vm.input.Entry.(type) {
	case *tx.Spend:
		oEntry := e.SpentOutput().Entry
		if oEntry == nil {
			// xxx error
		}
		o, ok := oEntry.(*tx.Output)
		if !ok {
			// xxx error
		}
		amount = o.Amount()

	case *tx.Issuance:
		amount = e.Amount()

	default:
		// xxx error
	}

	return vm.pushInt64(int64(amount), true)
}

func opProgram(vm *virtualMachine) error {
	if vm.txHeaderRef.IsNil() {
		return ErrContext
	}

	err := vm.applyCost(1)
	if err != nil {
		return err
	}

	return vm.push(vm.mainprog, true)
}

func opMinTime(vm *virtualMachine) error {
	if vm.txHeaderRef.IsNil() {
		return ErrContext
	}

	err := vm.applyCost(1)
	if err != nil {
		return err
	}

	hdr := vm.txHeaderRef.Entry.(*tx.Header)
	return vm.pushInt64(int64(hdr.MinTimeMS()), true)
}

func opMaxTime(vm *virtualMachine) error {
	if vm.txHeaderRef.IsNil() {
		return ErrContext
	}

	err := vm.applyCost(1)
	if err != nil {
		return err
	}

	hdr := vm.txHeaderRef.Entry.(*tx.Header)
	maxTime := hdr.MaxTimeMS()
	if maxTime == 0 || maxTime > math.MaxInt64 {
		maxTime = uint64(math.MaxInt64)
	}

	return vm.pushInt64(int64(maxTime), true)
}

func opRefDataHash(vm *virtualMachine) error {
	if vm.txHeaderRef.IsNil() {
		return ErrContext
	}

	err := vm.applyCost(1)
	if err != nil {
		return err
	}

	var h bc.Hash

	switch e := vm.input.Entry.(type) {
	case *tx.Spend:
		h, err = e.RefDataHash()
		if err != nil {
			return err
		}
	case *tx.Issuance:
		h, err = e.RefDataHash()
		if err != nil {
			return err
		}
	default:
		// xxx error
	}

	return vm.push(h[:], true)
}

func opTxRefDataHash(vm *virtualMachine) error {
	if vm.txHeaderRef.IsNil() {
		return ErrContext
	}

	err := vm.applyCost(1)
	if err != nil {
		return err
	}

	hdr := vm.txHeaderRef.Entry.(*tx.Header)
	h := hdr.RefDataHash()
	return vm.push(h[:], true)
}

func opOutputID(vm *virtualMachine) error {
	if vm.txHeaderRef.IsNil() {
		return ErrContext
	}

	sp, ok := vm.input.Entry.(*tx.Spend)
	if !ok {
		return ErrContext
	}
	if sp == nil {
		// xxx error
	}
	spent := sp.SpentOutput()
	outID, err := spent.Hash()
	if err != nil {
		return err
	}

	err = vm.applyCost(1)
	if err != nil {
		return err
	}

	return vm.push(outID[:], true)
}

func opNonce(vm *virtualMachine) error {
	if vm.txHeaderRef.IsNil() {
		return ErrContext
	}

	_, ok := vm.input.Entry.(*tx.Issuance)
	if !ok {
		return ErrContext
	}

	err := vm.applyCost(1)
	if err != nil {
		return err
	}

	var nonce []byte
	// xxx
	return vm.push(nonce, true)
}

func opNextProgram(vm *virtualMachine) error {
	if vm.block == nil {
		return ErrContext
	}
	err := vm.applyCost(1)
	if err != nil {
		return err
	}
	return vm.push(vm.block.ConsensusProgram, true)
}

func opBlockTime(vm *virtualMachine) error {
	if vm.block == nil {
		return ErrContext
	}
	err := vm.applyCost(1)
	if err != nil {
		return err
	}
	if vm.block.TimestampMS > math.MaxInt64 {
		return fmt.Errorf("block timestamp out of range")
	}
	return vm.pushInt64(int64(vm.block.TimestampMS), true)
}
