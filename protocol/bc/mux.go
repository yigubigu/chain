package bc

import "io"

type mux struct {
	body struct {
		Sources []valueSource
	}
}

const typeMux = "mux1"

func (mux) Type() string         { return typeMux }
func (m *mux) Body() interface{} { return m.body }

func newMux(sources []valueSource) *mux {
	m := new(mux)
	m.body.Sources = sources
	return m
}

func (m *mux) WriteTo(w io.Writer) (int64, error) {
	// xxx
}

func (m *mux) ReadFrom(r io.Reader) (int64, error) {
	// xxx
}
