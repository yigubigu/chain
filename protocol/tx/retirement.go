package tx

import "chain/protocol/bc"

type Retirement struct {
	body struct {
		Source  valueSource
		DataRef bc.Hash
		ExtHash extHash
	}
	ordinal int
}

func (Retirement) Type() string         { return "retirement1" }
func (r *Retirement) Body() interface{} { return r.body }

func (r Retirement) Ordinal() int { return r.ordinal }

func newRetirement(source valueSource, dataRef bc.Hash, ordinal int) *Retirement {
	r := new(Retirement)
	r.body.Source = source
	r.body.DataRef = dataRef
	r.ordinal = ordinal
	return r
}
