package bc

import (
	"chain/crypto/sha3pool"
)

// A "Data" entry represents some arbitrary data
// the transaction author wants the current transaction to commit to,
// either for use in programs in the current or future transactions,
// or for reference by external systems.
// This is done with a hash commitment:
// the entry itself stores a 32-byte hash of the underlying data,
// which may be of any length.
// It is the responsibility of the transport layer
// to provide the underlying data
// alongside the actual transaction, if necessary.
// The data need not be made available to all parties;
// it is fine to keep it confidential.
//
// Note that the body of this entry is a hash (of the underlying data);
// when a Data entry is hashed, its body_hash is a hash of that hash.
type Data struct {
	body Hash
}

func (Data) Type() string         { return "data1" }
func (d *Data) Body() interface{} { return d.body }

func NewData(hash Hash) *Data {
	d := new(Data)
	d.body = hash
	return d
}

// refDataHash returns the data hash in e, which must be nil or a data
// entry pointer. If e is nil (or e.IsNil() is true) the result is the
// hash of the empty string.
func refDataHash(e *EntryRef) Hash {
	if e == nil || e.Entry == nil {
		return EmptyStringHash
	}
	return e.Entry.(*data).body
}

func HashData(data []byte) (h Hash) {
	sha3pool.Sum256(h[:], data)
	return
}
