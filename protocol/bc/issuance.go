package bc

type Issuance struct {
	body struct {
		Anchor  *EntryRef
		Value   AssetAmount
		Data    *EntryRef
		ExtHash extHash
	}
	witness struct {
		Destination     valueDestination
		InitialBlockID  Hash
		AssetDefinition *EntryRef // data entry
		IssuanceProgram Program
		Arguments       [][]byte
		ExtHash         extHash
	}
}

func (Issuance) Type() string           { return "issuance1" }
func (iss *Issuance) Body() interface{} { return iss.body }

func (iss *Issuance) AssetID() AssetID {
	return iss.body.Value.AssetID
}

func (iss *Issuance) Amount() uint64 {
	return iss.body.Value.Amount
}

func (iss *Issuance) Anchor() *EntryRef {
	return iss.body.Anchor
}

func (iss *Issuance) RefDataHash() Hash {
	return refDataHash(iss.body.Data)
}

func (iss *Issuance) IssuanceProgram() Program {
	return iss.witness.IssuanceProgram
}

func (iss *Issuance) Arguments() [][]byte {
	return iss.witness.Arguments
}

func newIssuance(anchor *EntryRef, value AssetAmount, data *EntryRef) *Issuance {
	iss := new(Issuance)
	iss.body.Anchor = anchor
	iss.body.Value = value
	iss.body.Data = data
	return iss
}
