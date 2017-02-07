package tx

type Retirement struct {
	body struct {
		Source  valueSource
		Data    EntryRef
		ExtHash extHash
	}
}

func (Retirement) Type() string         { return "retirement1" }
func (r *Retirement) Body() interface{} { return r.body }

func newRetirement(source valueSource, data EntryRef) *Retirement {
	r := new(Retirement)
	r.body.Source = source
	r.body.Data = data
	return r
}
