package bc

import (
	"chain/encoding/blockchain"
	"io"
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

// Inputs returns all input entries (as two lists: spends and
// issuances) reachable from a header's result entries.
func (h *Header) Inputs() (spends, issuances []*EntryRef, err error) {
	sMap := make(map[Hash]*EntryRef)
	iMap := make(map[Hash]*EntryRef)

	// Declare accum before assigning it, so it can reference itself
	// recursively.
	var accum func(*EntryRef) error
	accum = func(ref *EntryRef) error {
		switch e := ref.Entry.(type) {
		case *Spend:
			hash, err := ref.Hash()
			if err != nil {
				return err
			}
			sMap[hash] = ref

		case *Issuance:
			hash, err := ref.Hash()
			if err != nil {
				return err
			}
			iMap[hash] = ref

		case *mux:
			for _, s := range e.body.Sources {
				accum(s.Ref)
			}
		}
		return nil
	}

	for _, r := range h.body.Results {
		err = accum(r)
		if err != nil {
			return nil, nil, err
		}
	}

	for _, e := range sMap {
		spends = append(spends, e)
	}
	for _, e := range iMap {
		issuances = append(issuances, e)
	}
	return spends, issuances, nil
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
