package tx

import (
	"fmt"
	"io"
	"reflect"

	"chain/crypto/sha3pool"
	"chain/encoding/blockchain"
	"chain/errors"
	"chain/protocol/bc"
)

type (
	Entry interface {
		Type() string
		Body() interface{}
	}

	// EntryRef holds one or both of an entry and its id. If the entry
	// is present and the id is not, the id can be generated (and then
	// cached) on demand. Both may also be nil to represent a nil entry
	// pointer.
	EntryRef struct {
		Entry
		ID *bc.Hash
	}

	hasher interface {
		Hash() (bc.Hash, error)
	}
)

// Hash returns the EntryRef's cached entry ID, computing it first if
// necessary. Satisfies the hasher interface.
func (r *EntryRef) Hash() (bc.Hash, error) {
	if r.ID == nil {
		if r.Entry == nil {
			return bc.Hash{}, nil
		}
		h, err := entryID(r.Entry)
		if err != nil {
			return bc.Hash{}, err
		}
		r.ID = &h
	}
	return *r.ID, nil
}

func (r EntryRef) IsNil() bool {
	return r.Entry == nil && r.ID == nil
}

type extHash bc.Hash

var errInvalidValue = errors.New("invalid value")

func entryID(e Entry) (bc.Hash, error) {
	h := sha3pool.Get256()
	defer sha3pool.Put256(h)

	h.Write([]byte("entryid:"))
	h.Write([]byte(e.Type()))
	h.Write([]byte{':'})

	bh := sha3pool.Get256()
	defer sha3pool.Put256(bh)
	err := writeForHash(bh, e.Body())
	if err != nil {
		return bc.Hash{}, err
	}
	var innerHash bc.Hash
	bh.Read(innerHash[:])
	h.Write(innerHash[:])

	var hash bc.Hash
	h.Read(hash[:])

	return hash, nil
}

func writeForHash(w io.Writer, c interface{}) error {
	switch v := c.(type) {
	case hasher:
		h, err := v.Hash()
		if err != nil {
			return errors.Wrap(err, "computing hash")
		}
		_, err = w.Write(h[:])
		return errors.Wrap(err, "writing hash")
	case byte:
		_, err := w.Write([]byte{v})
		return errors.Wrap(err, "writing byte for hash")
	case uint64:
		_, err := blockchain.WriteVarint63(w, v)
		return errors.Wrapf(err, "writing uint64 (%d) for hash", v)
	case []byte:
		_, err := blockchain.WriteVarstr31(w, v)
		return errors.Wrapf(err, "writing []byte (len %d) for hash", len(v))
	case string:
		_, err := blockchain.WriteVarstr31(w, []byte(v))
		return errors.Wrapf(err, "writing string (len %d) for hash", len(v))

		// TODO: The rest of these are all aliases for [32]byte. Do we
		// really need them all?

	case bc.Hash:
		_, err := w.Write(v[:])
		return errors.Wrap(err, "writing bc.Hash for hash")
	case extHash:
		_, err := w.Write(v[:])
		return errors.Wrap(err, "writing extHash for hash")
	case bc.AssetID:
		_, err := w.Write(v[:])
		return errors.Wrap(err, "writing bc.AssetID for hash")
	}

	// The two container types in the spec (List and Struct)
	// correspond to slices and structs in Go. They can't be
	// handled with type assertions, so we must use reflect.
	switch v := reflect.ValueOf(c); v.Kind() {
	case reflect.Slice:
		l := v.Len()
		_, err := blockchain.WriteVarint31(w, uint64(l))
		if err != nil {
			return errors.Wrapf(err, "writing slice (len %d) for hash", l)
		}
		for i := 0; i < l; i++ {
			c := v.Index(i)
			if !c.CanInterface() {
				return errInvalidValue
			}
			err := writeForHash(w, c.Interface())
			if err != nil {
				return errors.Wrapf(err, "writing slice element %d for hash", i)
			}
		}
		return nil

	case reflect.Struct:
		for i := 0; i < v.NumField(); i++ {
			c := v.Field(i)
			if !c.CanInterface() {
				return errInvalidValue
			}
			err := writeForHash(w, c.Interface())
			if err != nil {
				t := v.Type()
				f := t.Field(i)
				return errors.Wrapf(err, "writing struct field %d (%s.%s) for hash", i, t.Name(), f.Name)
			}
		}
		return nil
	}

	return errors.Wrap(fmt.Errorf("bad type %T", c))
}
