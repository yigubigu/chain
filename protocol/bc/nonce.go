package bc

type Nonce struct {
	body struct {
		Program   Program
		TimeRange *EntryRef
		ExtHash   extHash
	}
}

func (Nonce) Type() string         { return "nonce1" }
func (n *Nonce) Body() interface{} { return n.body }

func newNonce(p Program, tr *EntryRef) *Nonce {
	n := new(Nonce)
	n.body.Program = p
	n.body.TimeRange = tr
	return n
}
