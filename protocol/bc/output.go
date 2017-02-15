package bc

import "io"

type Output struct {
	body struct {
		Source         valueSource
		ControlProgram Program
		Data           *EntryRef
		ExtHash        extHash
	}
}

const typeOutput = "output1"

func (Output) Type() string         { return typeOutput }
func (o *Output) Body() interface{} { return o.body }

func (o *Output) AssetID() AssetID {
	return o.body.Source.Value.AssetID
}

func (o *Output) Amount() uint64 {
	return o.body.Source.Value.Amount
}

func (o *Output) ControlProgram() Program {
	return o.body.ControlProgram
}

func (o *Output) RefDataHash() Hash {
	return refDataHash(o.body.Data)
}

func newOutput(source valueSource, controlProgram Program, data *EntryRef) *Output {
	out := new(Output)
	out.body.Source = source
	out.body.ControlProgram = controlProgram
	out.body.Data = data
	return out
}

func (o *Output) WriteTo(w io.Writer) (int64, error) {
	// xxx
}

func (o *Output) ReadFrom(r io.Reader) (int64, error) {
	// xxx
}
