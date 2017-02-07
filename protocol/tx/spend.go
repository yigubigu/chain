package tx

import "chain/protocol/bc"

type Spend struct {
	body struct {
		SpentOutput bc.OutputID
		DataRef     bc.Hash
		ExtHash     extHash
	}
	ordinal int
}

func (Spend) Type() string         { return "spend1" }
func (s *Spend) Body() interface{} { return s.body }

func (s Spend) Ordinal() int { return s.ordinal }

func newSpend(spentOutput bc.OutputID, dataRef bc.Hash, ordinal int) *Spend {
	s := new(Spend)
	s.body.SpentOutput = spentOutput
	s.body.DataRef = dataRef
	s.ordinal = ordinal
	return s
}
