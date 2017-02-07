package tx

import "chain/protocol/bc"

type (
	// output contains a valueSource that must refer to the mux by its
	// entryID, but the mux may not be complete at the time AddOutput is
	// called, so we hold outputs in a pending structure until Build is
	// called
	pendingOutput struct {
		value       bc.AssetAmount
		controlProg bc.Program
		data        EntryRef
	}

	pendingRetirement struct {
		value bc.AssetAmount
		data  EntryRef
	}

	Builder struct {
		h           *Header
		m           *mux
		outputs     []*pendingOutput
		retirements []*pendingRetirement
		entries     map[bc.Hash]Entry
	}
)

func NewBuilder(version, minTimeMS, maxTimeMS uint64) *Builder {
	return &Builder{
		h:       newHeader(version, nil, EntryRef{}, minTimeMS, maxTimeMS),
		m:       newMux(nil),
		entries: make(map[bc.Hash]Entry),
	}
}

func (b *Builder) AddData(h bc.Hash) (*Builder, *data, bc.Hash) {
	d := newData(h)
	dID := mustEntryID(d)
	// xxx b.h.body.Data = dID?
	b.entries[dID] = d
	return b, d, dID
}

func (b *Builder) AddIssuance(nonce EntryRef, value bc.AssetAmount, data EntryRef) (*Builder, EntryRef) {
	iss := newIssuance(nonce, value, data)
	issID := mustEntryID(iss)
	s := valueSource{
		Ref:   EntryRef{Entry: iss, ID: &issID},
		Value: value,
	}
	b.m.body.Sources = append(b.m.body.Sources, s)
	b.entries[issID] = iss
	return b, EntryRef{Entry: iss, ID: &issID}
}

func (b *Builder) AddNonce(p bc.Program, timeRange EntryRef) (*Builder, EntryRef) {
	n := newNonce(p, timeRange)
	nID := mustEntryID(n)
	b.entries[nID] = n
	return b, EntryRef{Entry: n, ID: &nID}
}

// AddOutput returns only the builder, unlike most other Add
// functions, since output objects aren't created until Build
func (b *Builder) AddOutput(value bc.AssetAmount, controlProg bc.Program, data EntryRef) *Builder {
	b.outputs = append(b.outputs, &pendingOutput{
		value:       value,
		controlProg: controlProg,
		data:        data,
	})
	return b
}

// AddRetirement returns only the builder, unlike most other Add
// functions, since retirement objects aren't created until Build
func (b *Builder) AddRetirement(value bc.AssetAmount, data EntryRef) *Builder {
	b.retirements = append(b.retirements, &pendingRetirement{
		value: value,
		data:  data,
	})
	return b
}

func (b *Builder) AddSpend(spentOutput EntryRef, value bc.AssetAmount, data EntryRef) (*Builder, EntryRef) {
	sp := newSpend(spentOutput, data)
	spID := mustEntryID(sp)
	src := valueSource{
		Ref:   EntryRef{Entry: sp, ID: &spID},
		Value: value,
	}
	b.m.body.Sources = append(b.m.body.Sources, src)
	b.entries[spID] = sp
	return b, EntryRef{Entry: sp, ID: &spID}
}

func (b *Builder) AddTimeRange(minTimeMS, maxTimeMS uint64) (*Builder, EntryRef) {
	tr := newTimeRange(minTimeMS, maxTimeMS)
	trID := mustEntryID(tr)
	b.entries[trID] = tr
	return b, EntryRef{Entry: tr, ID: &trID}
}

func (b *Builder) Build() (bc.Hash, *Header, map[bc.Hash]Entry) {
	muxID := mustEntryID(b.m)
	b.entries[muxID] = b.m
	var n uint64
	for _, po := range b.outputs {
		s := valueSource{
			Ref:      EntryRef{Entry: b.m, ID: &muxID},
			Value:    po.value,
			Position: n,
		}
		n++
		o := newOutput(s, po.controlProg, po.data)
		oID := mustEntryID(o)
		b.entries[oID] = o
		b.h.body.Results = append(b.h.body.Results, EntryRef{Entry: o, ID: &oID})
	}
	for _, pr := range b.retirements {
		s := valueSource{
			Ref:      EntryRef{Entry: b.m, ID: &muxID},
			Value:    pr.value,
			Position: n,
		}
		n++
		r := newRetirement(s, pr.data)
		rID := mustEntryID(r)
		b.entries[rID] = r
		b.h.body.Results = append(b.h.body.Results, EntryRef{Entry: r, ID: &rID})
	}
	hID := mustEntryID(b.h)
	b.entries[hID] = b.h
	return hID, b.h, b.entries
}

func mustEntryID(e Entry) bc.Hash {
	res, err := entryID(e)
	if err != nil {
		panic(err)
	}
	return res
}
