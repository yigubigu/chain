package tx

import "chain/protocol/bc"

type Nonce struct {
	body struct {
		Program   bc.Program
		TimeRange EntryRef
		ExtHash   extHash
	}
}

func (Nonce) Type() string         { return "nonce1" }
func (n *Nonce) Body() interface{} { return n.body }

func newNonce(p bc.Program, tr EntryRef) *Nonce {
	n := new(Nonce)
	n.body.Program = p
	n.body.TimeRange = tr
	return n
}
