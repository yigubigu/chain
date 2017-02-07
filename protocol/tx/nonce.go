package tx

import "chain/protocol/bc"

type Nonce struct {
	body struct {
		Program      bc.Program
		TimeRangeRef bc.Hash
		ExtHash      extHash
	}
}

func (Nonce) Type() string         { return "nonce1" }
func (n *Nonce) Body() interface{} { return n.body }

func (Nonce) Ordinal() int { return -1 }

func newNonce(p bc.Program, trRef bc.Hash) *Nonce {
	n := new(Nonce)
	n.body.Program = p
	n.body.TimeRangeRef = trRef
	return n
}
