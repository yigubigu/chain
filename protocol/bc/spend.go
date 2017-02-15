package bc

import "io"

type Spend struct {
	body struct {
		SpentOutput *EntryRef
		Data        *EntryRef
		ExtHash     extHash
	}
	witness struct {
		Destination valueDestination
		Arguments   [][]byte
		ExtHash     extHash
	}
}

const typeSpend = "spend1"

func (Spend) Type() string         { return typeSpend }
func (s *Spend) Body() interface{} { return s.body }

func (s *Spend) SpentOutput() *EntryRef {
	return s.body.SpentOutput
}

func (s *Spend) RefDataHash() Hash {
	return refDataHash(s.body.Data)
}

func (s *Spend) Arguments() [][]byte {
	return s.witness.Arguments
}

func newSpend(spentOutput, data *EntryRef) *Spend {
	s := new(Spend)
	s.body.SpentOutput = spentOutput
	s.body.Data = data
	return s
}

func (s *Spend) WriteTo(w io.Writer) (int64, error) {
	// xxx
}

func (s *Spend) ReadFrom(r io.Reader) (int64, error) {
	// xxx
}
