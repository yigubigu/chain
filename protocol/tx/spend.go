package tx

import "chain/protocol/bc"

type Spend struct {
	body struct {
		SpentOutput EntryRef
		Data        EntryRef
		ExtHash     extHash
	}
	witness struct {
		Destination valueDestination
		Arguments   [][]byte
		ExtHash     extHash
	}
}

func (Spend) Type() string         { return "spend1" }
func (s *Spend) Body() interface{} { return s.body }

func (s *Spend) SpentOutput() EntryRef {
	return s.body.SpentOutput
}

func (s *Spend) RefDataHash() (bc.Hash, error) {
	dEntry := s.body.Data.Entry
	if dEntry == nil {
		// xxx error
	}
	d, ok := dEntry.(*data)
	if !ok {
		// xxx error
	}
	return d.body, nil
}

func (s *Spend) Arguments() [][]byte {
	return s.witness.Arguments
}

func newSpend(spentOutput, data EntryRef) *Spend {
	s := new(Spend)
	s.body.SpentOutput = spentOutput
	s.body.Data = data
	return s
}
