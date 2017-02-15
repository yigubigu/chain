package bc

import (
	"io"

	"chain/encoding/blockchain"
)

type Header struct {
	body struct {
		Version              uint64
		Results              []*EntryRef
		Data                 *EntryRef
		MinTimeMS, MaxTimeMS uint64
		ExtHash              extHash
	}
}

const typeHeader = "txheader"

func (Header) Type() string         { return typeHeader }
func (h *Header) Body() interface{} { return h.body }

func (h *Header) Version() uint64 {
	return h.body.Version
}

func (h *Header) MinTimeMS() uint64 {
	return h.body.MinTimeMS
}

func (h *Header) MaxTimeMS() uint64 {
	return h.body.MaxTimeMS
}

func (h *Header) Results() []*EntryRef {
	return h.body.Results
}

func (h *Header) RefDataHash() Hash {
	return refDataHash(h.body.Data)
}

func (h *Header) Walk(visitor func(*EntryRef) error) error {
	visited := make(map[Hash]bool)
	visit := func(e *EntryRef) error {
		if e == nil {
			return
		}
		if visited[e.Hash()] {
			return
		}
		visited[e.Hash()] = true
		return visitor(e)
	}
	err := visit(h.body.Data)
	if err != nil {
		return err
	}
	for _, res := range h.body.Results {
		err = visit(res)
		if err != nil {
			return err
		}
		switch e2 := res.Entry.(type) {
		case *Issuance:
			err = visit(e2.body.Anchor)
			if err != nil {
				return err
			}
			err = visit(e2.body.EntryRef)
			if err != nil {
				return err
			}
			err = visit(e2.witness.Destination.Ref)
			if err != nil {
				return err
			}
			err = visit(e2.witness.AssetDefinition)
			if err != nil {
				return err
			}
		case *mux:
			for _, vs := range e2.body.Sources {
				err = visit(vs.Ref)
				if err != nil {
					return err
				}
			}
		case *Nonce:
			err = visit(e2.body.TimeRange)
			if err != nil {
				return err
			}
		case *Output:
			err = visit(e2.body.Source.Ref)
			if err != nil {
				return err
			}
			err = visit(e2.body.Data)
			if err != nil {
				return err
			}
		case *Retirement:
			err = visit(e2.body.Source.Ref)
			if err != nil {
				return err
			}
			err = visit(e2.body.Data)
			if err != nil {
				return err
			}
		case *Spend:
			err = visit(e2.body.SpentOutput)
			if err != nil {
				return err
			}
			err = visit(e2.body.Data)
			if err != nil {
				return err
			}
			err = visit(e2.witness.Destination.Ref)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

// Inputs returns all input entries (as two lists: spends and
// issuances) reachable from a header's result entries.
func (h *Header) Inputs() (spends, issuances []*EntryRef) {
	h.Walk(func(e *EntryRef) error {
		switch e2 := e.Entry.(type) {
		case *Spend:
			spends = append(spends, e)
		case *Issuance:
			issuances = append(issuances, e)
		}
		return nil
	})
	return
}

func newHeader(version uint64, results []*EntryRef, data *EntryRef, minTimeMS, maxTimeMS uint64) *Header {
	h := new(Header)
	h.body.Version = version
	h.body.Results = results
	h.body.Data = data
	h.body.MinTimeMS = minTimeMS
	h.body.MaxTimeMS = maxTimeMS
	return h
}

func (h *Header) WriteTo(w io.Writer) (int64, error) {
	n, err := blockchain.WriteVarint63(w, h.body.Version)
	if err != nil {
		return int64(n), err
	}
	n2, err := blockchain.WriteVarint63(w, h.body.MinTimeMS)
	n += n2
	if err != nil {
		return int64(n), err
	}
	n2, err = blockchain.WriteVarint63(w, h.body.MaxTimeMS)
	n += n2
	if err != nil {
		return int64(n), err
	}
}

func (h *Header) ReadFrom(r io.Reader) (int64, error) {
	// xxx
}

func (h *Header) WriteTxTo(w io.Writer) (int64, error) {
	var entries []*EntryRef
	h.Walk(func(e *EntryRef) error {
		if e.Entry != nil {
			entries = append(entries, e)
		}
	})
	n, err := h.WriteTo(w)
	if err != nil {
		return n, err
	}
	n2, err := blockchain.WriteVarint31(w, uint64(len(entries)))
	n += int64(n2)
	if err != nil {
		return n, err
	}
	for _, e := range entries {
		n3, err := e.WriteEntry(w)
		n += n3
		if err != nil {
			return n, err
		}
	}
	return n, nil
}
