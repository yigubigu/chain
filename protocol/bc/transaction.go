package bc

type Transaction struct {
	Header      *EntryRef
	Issuances   []*EntryRef
	Spends      []*EntryRef
	Outputs     []*EntryRef
	Retirements []*EntryRef
}

func NewTransaction(hdrRef *EntryRef) *Transaction {
	hdr := hdrRef.Entry.(*Header)
	spends, issuances := hdr.Inputs()
	tx := &Transaction{
		Header:    hdrRef,
		Issuances: issuances,
		Spends:    spends,
	}
	for _, r := range hdr.Results() {
		switch r.Entry.(type) {
		case *Output:
			tx.Outputs = append(tx.Outputs, r)
		case *Retirement:
			tx.Retirements = append(tx.Retirements, r)
		}
	}
	return tx
}
