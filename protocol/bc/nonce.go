package bc

type Nonce struct {
	body struct {
		Program   Program
		TimeRange *EntryRef
		ExtHash   extHash
	}
}

const typeNonce = "nonce1"

func (Nonce) Type() string            { return typeNonce }
func (n *Nonce) Body() interface{}    { return &n.body }
func (n *Nonce) Witness() interface{} { return nil }

func newNonce(p Program, tr *EntryRef) *Nonce {
	n := new(Nonce)
	n.body.Program = p
	n.body.TimeRange = tr
	return n
}
