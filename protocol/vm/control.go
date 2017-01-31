package vm

import (
	"encoding/binary"
)

func opVerify(vm *virtualMachine) error {
	err := vm.applyCost(1)
	if err != nil {
		return err
	}
	p, err := vm.pop(true)
	if err != nil {
		return err
	}
	if AsBool(p) {
		return nil
	}
	return ErrVerifyFailed
}

func opFail(vm *virtualMachine) error {
	err := vm.applyCost(1)
	if err != nil {
		return err
	}
	return ErrReturn
}

func opCheckPredicate(vm *virtualMachine) error {
	err := vm.applyCost(256)
	if err != nil {
		return err
	}
	vm.deferCost(-256 + 64) // get most of that cost back at the end
	limit, err := vm.popInt64(true)
	if err != nil {
		return err
	}
	predicate, err := vm.pop(true)
	if err != nil {
		return err
	}
	n, err := vm.popInt64(true)
	if err != nil {
		return err
	}
	if limit < 0 {
		return ErrBadValue
	}
	l := int64(len(vm.dataStack))
	if n > l {
		return ErrDataStackUnderflow
	}
	if limit == 0 {
		limit = vm.runLimit
	}
	err = vm.applyCost(limit)
	if err != nil {
		return err
	}

	childVM := virtualMachine{
		mainprog:   vm.mainprog,
		program:    predicate,
		runLimit:   limit,
		depth:      vm.depth + 1,
		dataStack:  append([][]byte{}, vm.dataStack[l-n:]...),
		tx:         vm.tx,
		inputIndex: vm.inputIndex,
		sigHasher:  vm.sigHasher,
	}
	vm.dataStack = vm.dataStack[:l-n]

	childErr := childVM.run()

	vm.deferCost(-childVM.runLimit)
	vm.deferCost(-stackCost(childVM.dataStack))
	vm.deferCost(-stackCost(childVM.altStack))

	return vm.pushBool(childErr == nil && !childVM.falseResult(), true)
}

func opJump(vm *virtualMachine) error {
	err := vm.applyCost(1)
	if err != nil {
		return err
	}
	address := binary.LittleEndian.Uint32(vm.data)
	vm.nextPC = address
	return nil
}

func opJumpIf(vm *virtualMachine) error {
	err := vm.applyCost(1)
	if err != nil {
		return err
	}
	p, err := vm.pop(true)
	if err != nil {
		return err
	}
	if AsBool(p) {
		address := binary.LittleEndian.Uint32(vm.data)
		vm.nextPC = address
	}
	return nil
}

func opThreshold(vm *virtualMachine) error {
	// xxx entry cost
	// xxx deferred exit cost/refund
	nPredicates, err := vm.popInt64(true)
	if err != nil {
		return err
	}
	// xxx range-check nPredicates
	nArgs, err := vm.popInt64(true)
	if err != nil {
		return err
	}
	// xxx range-check nArgs
	predicates := make([][]byte, 0, nPredicates)
	for i := int64(0); i < nPredicates; i++ {
		p, err := vm.pop(true)
		if err != nil {
			return err
		}
		predicates = append(predicates, p)
	}
	mask, err := vm.pop(true)
	if err != nil {
		return err
	}
	// xxx count of 1-bits in mask must == nArgs
	// xxx index of highest 1-bit in mask must be < nPredicates
	args := make([][]byte, 0, nArgs)
	for i := int64(0); i < nArgs; i++ {
		a, err := vm.pop(true)
		if err != nil {
			return err
		}
		args = append(args, a)
	}
	for i := int64(0); i < nPredicates; i++ {
		byteNum := i / 8
		bytePos := uint(i % 8)
		if mask[byteNum]&(1<<bytePos) != 0 {
			arg := args[0]
			args = args[1:]
			// xxx childvm setup cost
			childVM := virtualMachine{
				mainprog: vm.mainprog,
				program:  predicates[i],
				// xxx runLimit
				depth:      vm.depth + 1,
				dataStack:  [][]byte{arg},
				tx:         vm.tx,
				inputIndex: vm.inputIndex,
				sigHasher:  vm.sigHasher,
			}
			childErr := childVM.run()
			// xxx childvm teardown cost/refund
			if childErr != nil || childVM.falseResult() {
				// short circuit
				return vm.pushBool(false, true)
			}
		}
	}
	return vm.pushBool(true, true)
}
