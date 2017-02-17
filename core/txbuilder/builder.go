package txbuilder

import (
	"bytes"
	"math"
	"time"

	"chain/errors"
	"chain/protocol/bc"
)

func NewBuilder(maxTime time.Time, tx *bc.EntryRef) *TemplateBuilder {
	return &TemplateBuilder{
		bcBuilder:           bc.NewBuilder(1, bc.Millis(time.Now()), bc.Millis(maxTime), tx),
		signingInstructions: make(map[bc.Hash]*SigningInstruction),
	}
}

type TemplateBuilder struct {
	bcBuilder           *bc.Builder
	signingInstructions map[bc.Hash]*SigningInstruction
	rollbacks           []func()
	callbacks           []func() error
}

func (b *TemplateBuilder) AddSpend(spentOutput *bc.EntryRef, value bc.AssetAmount, data *bc.EntryRef, sigInstruction *SigningInstruction) error {
	if value.Amount > math.MaxInt64 {
		return errors.WithDetailf(ErrBadAmount, "amount %d exceeds maximum value 2^63", value.Amount)
	}
	if prevout, ok := spentOutput.Entry.(*bc.Output); ok {
		if value.AssetID != prevout.AssetID() || value.Amount != prevout.Amount() {
			return errors.WithDetailf(ErrBadAmount, "amount %d of asset %x does not match the output being spent", value.Amount, value.AssetID[:])
		}
	}
	spRef := b.bcBuilder.AddSpend(spentOutput, value, data)
	b.signingInstructions[spRef.Hash()] = sigInstruction
	return nil
}

func (b *TemplateBuilder) AddIssuance(nonce *bc.EntryRef, value bc.AssetAmount, data *bc.EntryRef, sigInstruction *SigningInstruction) error {
	if value.Amount > math.MaxInt64 {
		return errors.WithDetailf(ErrBadAmount, "amount %d exceeds maximum value 2^63", value.Amount)
	}
	issRef := b.bcBuilder.AddIssuance(nonce, value, data)
	b.signingInstructions[issRef.Hash()] = sigInstruction
	return nil
}

func (b *TemplateBuilder) AddOutput(value bc.AssetAmount, controlProg bc.Program, data *bc.EntryRef) error {
	if value.Amount > math.MaxInt64 {
		return errors.WithDetailf(ErrBadAmount, "amount %d exceeds maximum value 2^63", value.Amount)
	}
	b.bcBuilder.AddOutput(value, controlProg, data)
	return nil
}

func (b *TemplateBuilder) AddRetirement(value bc.AssetAmount, data *bc.EntryRef) error {
	if value.Amount > math.MaxInt64 {
		return errors.WithDetailf(ErrBadAmount, "amount %d exceeds maximum value 2^63", value.Amount)
	}
	b.bcBuilder.AddRetirement(value, data)
	return nil
}

func (b *TemplateBuilder) RestrictMinTime(t time.Time) {
	if t.After(b.minTime) {
		b.minTime = t
	}
}

func (b *TemplateBuilder) RestrictMaxTime(t time.Time) {
	if t.Before(b.maxTime) {
		b.maxTime = t
	}
}

func (b *TemplateBuilder) MaxTime() time.Time {
	return b.maxTime
}

// OnRollback registers a function that can be
// used to attempt to undo any side effects of building
// actions. For example, it might cancel any reservations
// reservations that were made on UTXOs in a spend action.
// Rollback is a "best-effort" operation and not guaranteed
// to succeed. Each action's side effects, if any, must be
// designed with this in mind.
func (b *TemplateBuilder) OnRollback(rollbackFn func()) {
	b.rollbacks = append(b.rollbacks, rollbackFn)
}

// OnBuild registers a function that will be run after all
// actions have been successfully built.
func (b *TemplateBuilder) OnBuild(buildFn func() error) {
	b.callbacks = append(b.callbacks, buildFn)
}

func (b *TemplateBuilder) setReferenceData(data []byte) error {
	// if b.base != nil && len(b.base.ReferenceData) != 0 && !bytes.Equal(b.base.ReferenceData, data) {
	// 	return errors.Wrap(ErrBadRefData)
	// }
	// xxx update the following
	if len(b.referenceData) != 0 && !bytes.Equal(b.referenceData, data) {
		return errors.Wrap(ErrBadRefData)
	}
	b.referenceData = data
	return nil
}

func (b *TemplateBuilder) rollback() {
	for _, f := range b.rollbacks {
		f()
	}
}

func (b *TemplateBuilder) Build() (*Template, *bc.TxData, error) {
	// Run any building callbacks.
	for _, cb := range b.callbacks {
		err := cb()
		if err != nil {
			return nil, nil, err
		}
	}

	tpl := &Template{}
	tx := b.base
	if tx == nil {
		tx = &bc.TxData{
			Version: bc.CurrentTransactionVersion,
		}
		tpl.Local = true
	}

	// Update min & max times.
	if !b.minTime.IsZero() && bc.Millis(b.minTime) > tx.MinTime {
		tx.MinTime = bc.Millis(b.minTime)
	}
	if tx.MaxTime == 0 || tx.MaxTime > bc.Millis(b.maxTime) {
		tx.MaxTime = bc.Millis(b.maxTime)
	}

	// Set transaction reference data if applicable.
	if len(b.referenceData) > 0 {
		tx.ReferenceData = b.referenceData
	}

	// Add all the built outputs.
	tx.Outputs = append(tx.Outputs, b.outputs...)

	// Add all the built inputs and their corresponding signing instructions.
	for i, in := range b.inputs {
		instruction := b.signingInstructions[i]
		instruction.Position = uint32(len(tx.Inputs))

		// Empty signature arrays should be serialized as empty arrays, not null.
		if instruction.WitnessComponents == nil {
			instruction.WitnessComponents = []WitnessComponent{}
		}
		tpl.SigningInstructions = append(tpl.SigningInstructions, instruction)
		tx.Inputs = append(tx.Inputs, in)
	}
	tpl.Transaction = bc.NewTx(*tx)
	return tpl, tx, nil
}
