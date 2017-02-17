package bc

type (
	// output contains a valueSource that must refer to the mux by its
	// entryID, but the mux may not be complete at the time AddOutput is
	// called, so we hold outputs in a pending structure until Build is
	// called
	pendingOutput struct {
		value       AssetAmount
		controlProg Program
		data        *EntryRef
	}

	pendingRetirement struct {
		value AssetAmount
		data  *EntryRef
	}

	Builder struct {
		h                 *Header
		m                 *mux
		spends, issuances []*EntryRef
		outputs           []*pendingOutput
		retirements       []*pendingRetirement
	}
)

func NewBuilder(version, minTimeMS, maxTimeMS uint64, base *EntryRef) *Builder {
	result := &Builder{
		h: newHeader(version, nil, nil, minTimeMS, maxTimeMS),
		m: newMux(nil),
	}
	if base != nil {
		baseHdr := base.Entry.(*Header)
		entriesByHash := make(map[Hash]*EntryRef)
		var (
			spends, issuances, outputs, retirements []*EntryRef
		)
		baseHdr.Walk(func(e *EntryRef) error {
			entriesByHash[e.Hash()] = e
			switch e.Type() {
			case typeSpend:
				spends = append(spends, e)
			case typeIssuance:
				issuances = append(issuances, e)
			case typeOutput:
				outputs = append(outputs, e)
			case typeRetirement:
				retirements = append(retirements, e)
			}
			return nil
		})
		for _, e := range spends {
			sp := e.Entry.(*Spend)
			spentOutputRef := sp.SpentOutput()
			spentOutput := spentOutputRef.Entry.(*Output)
			result.AddSpend(spentOutputRef, AssetAmount{AssetID: spentOutput.AssetID(), Amount: spentOutput.Amount()}, sp.Data())
		}
		for _, e := range issuances {
			iss := e.Entry.(*Issuance)
			result.AddIssuance(iss.Anchor(), AssetAmount{AssetID: iss.AssetID(), Amount: iss.Amount()}, iss.Data())
		}
		for _, e := range outputs {
			o := e.Entry.(*Output)
			result.AddOutput(AssetAmount{AssetID: o.AssetID(), Amount: o.Amount()}, o.ControlProgram(), o.Data())
		}
		for _, e := range retirements {
			r := e.Entry.(*Retirement)
			result.AddRetirement(AssetAmount{AssetID: r.AssetID(), Amount: r.Amount()}, r.Data())
		}
	}
	return result
}

func (b *Builder) AddIssuance(nonce *EntryRef, value AssetAmount, data *EntryRef) *EntryRef {
	issRef := &EntryRef{Entry: newIssuance(nonce, value, data)}
	b.issuances = append(b.issuances, issRef)
	s := valueSource{
		Ref:   issRef,
		Value: value,
	}
	b.m.body.Sources = append(b.m.body.Sources, s)
	return issRef
}

// AddOutput does not return an entry, unlike other Add
// functions, since output objects aren't created until Build
func (b *Builder) AddOutput(value AssetAmount, controlProg Program, data *EntryRef) {
	b.outputs = append(b.outputs, &pendingOutput{
		value:       value,
		controlProg: controlProg,
		data:        data,
	})
}

// AddRetirement does not return an entry, unlike most other Add
// functions, since retirement objects aren't created until Build
func (b *Builder) AddRetirement(value AssetAmount, data *EntryRef) {
	b.retirements = append(b.retirements, &pendingRetirement{
		value: value,
		data:  data,
	})
}

func (b *Builder) AddSpend(spentOutput *EntryRef, value AssetAmount, data *EntryRef) *EntryRef {
	spRef := &EntryRef{Entry: newSpend(spentOutput, data)}
	b.spends = append(b.spends, spRef)
	src := valueSource{
		Ref:   spRef,
		Value: value,
	}
	b.m.body.Sources = append(b.m.body.Sources, src)
	return spRef
}

func (b *Builder) Build() *Transaction {
	var n uint64
	muxRef := &EntryRef{Entry: b.m}
	tx := &Transaction{
		Header:    &EntryRef{Entry: b.h},
		Spends:    b.spends,
		Issuances: b.issuances,
	}
	for _, po := range b.outputs {
		s := valueSource{
			Ref:      muxRef,
			Value:    po.value,
			Position: n,
		}
		n++
		o := newOutput(s, po.controlProg, po.data)
		oRef := &EntryRef{Entry: o}
		b.h.body.Results = append(b.h.body.Results, oRef)
		tx.Outputs = append(tx.Outputs, oRef)
	}
	for _, pr := range b.retirements {
		s := valueSource{
			Ref:      muxRef,
			Value:    pr.value,
			Position: n,
		}
		n++
		r := newRetirement(s, pr.data)
		rRef := &EntryRef{Entry: r}
		b.h.body.Results = append(b.h.body.Results, rRef)
		tx.Retirements = append(tx.Retirements, rRef)
	}
	return tx
}
