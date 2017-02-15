package bc

import "io"

type Retirement struct {
	body struct {
		Source  valueSource
		Data    *EntryRef
		ExtHash extHash
	}
}

const typeRetirement = "retirement1"

func (Retirement) Type() string         { return typeRetirement }
func (r *Retirement) Body() interface{} { return r.body }

func newRetirement(source valueSource, data *EntryRef) *Retirement {
	r := new(Retirement)
	r.body.Source = source
	r.body.Data = data
	return r
}

func (ret *Retirement) WriteTo(w io.Writer) (int64, error) {
	// xxx
}

func (ret *Retirement) ReadFrom(r io.Reader) (int64, error) {
	// xxx
}
