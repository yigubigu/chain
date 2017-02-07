package tx

import "chain/protocol/bc"

type Issuance struct {
	body struct {
		Anchor  EntryRef
		Value   bc.AssetAmount
		Data    EntryRef
		ExtHash extHash
	}
	witness struct {
		Destination     valueDestination
		InitialBlockID  bc.Hash
		AssetDefinition EntryRef // data entry
		IssuanceProgram bc.Program
		Arguments       [][]byte
		ExtHash         extHash
	}
}

func (Issuance) Type() string           { return "issuance1" }
func (iss *Issuance) Body() interface{} { return iss.body }

func (iss *Issuance) AssetID() bc.AssetID {
	return iss.body.Value.AssetID
}

func (iss *Issuance) Amount() uint64 {
	return iss.body.Value.Amount
}

func (iss *Issuance) Anchor() EntryRef {
	return iss.body.Anchor
}

func (iss *Issuance) RefDataHash() (bc.Hash, error) {
	dEntry := iss.body.Data.Entry
	if dEntry == nil {
		// xxx error
	}
	d, ok := dEntry.(*data)
	if !ok {
		// xxx error
	}
	return d.body, nil
}

func (iss *Issuance) IssuanceProgram() bc.Program {
	return iss.witness.IssuanceProgram
}

func (iss *Issuance) Arguments() [][]byte {
	return iss.witness.Arguments
}

func newIssuance(anchor EntryRef, value bc.AssetAmount, data EntryRef) *Issuance {
	iss := new(Issuance)
	iss.body.Anchor = anchor
	iss.body.Value = value
	iss.body.Data = data
	return iss
}
