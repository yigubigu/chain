package objhash

import (
	"encoding/binary"
	"fmt"
	"reflect"
	"sort"

	"chain/crypto/sha3pool"
)

type Hasher interface {
	Hash() [32]byte
}

// Hash produces a type-aware hash of a Go object.
// Unexported struct fields are excluded from hashing, recursively.
func Hash(obj interface{}) (hash [32]byte, err error) {
	if value, ok := obj.(reflect.Value); ok {
		if !value.CanInterface() {
			return hash, fmt.Errorf("cannot hash invalid value of type %s", value.Type().Name())
		}
		return Hash(value.Interface())
	}

	if hasher, ok := obj.(Hasher); ok {
		return hasher.Hash(), nil
	}

	t := reflect.TypeOf(obj)
	v := reflect.ValueOf(obj)

	var prefix string

	switch t.Kind() {
	case reflect.Bool:
		return hashBool(v.Bool()), nil
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		prefix = "int"
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
		prefix = "uint"
	case reflect.Float32, reflect.Float64:
		prefix = "float"
	case reflect.Complex64, reflect.Complex128:
		prefix = "complex"
	case reflect.Array:
		return hashArray(t, v)
	case reflect.Map:
		return hashMap(t, v)
	case reflect.Ptr:
		return hashPtr(t, v)
	case reflect.Slice:
		return hashSlice(t, v)
	case reflect.String:
		return hashString(v.String()), nil
	case reflect.Struct:
		return hashStruct(t, v)
		// Not supported:
		// case reflect.Chan:
		// case reflect.Func:
		// case reflect.Interface:
		// case reflect.UnsafePointer:
	}
	if prefix == "" {
		return hash, fmt.Errorf("unsupported kind %s", t.Kind())
	}
	h := sha3pool.Get256()
	defer sha3pool.Put256(h)
	h.Write([]byte(prefix))
	binary.Write(h, binary.LittleEndian, obj)
	h.Read(hash[:])
	return hash, nil
}

func hashBool(b bool) [32]byte {
	if b {
		return trueHash
	}
	return falseHash
}

func hashArray(t reflect.Type, v reflect.Value) (hash [32]byte, err error) {
	h := sha3pool.Get256()
	defer sha3pool.Put256(h)

	h.Write([]byte("array"))
	binary.Write(h, binary.LittleEndian, t.Len())
	for i := 0; i < t.Len(); i++ {
		subhash, err := Hash(v.Index(i))
		if err != nil {
			return hash, err
		}
		h.Write(subhash[:])
	}
	h.Read(hash[:])
	return hash, nil
}

type (
	kv struct {
		k, v [32]byte
	}
	kvs []*kv
)

func (kvs *kvs) Len() int           { return len(kvs) }
func (kvs *kvs) Less(i, j int) bool { return isHashLess(kvs[i].k, kvs[j].k) }
func (kvs *kvs) Swap(i, j int)      { kvs[i], kvs[j] = kvs[j], kvs[i] }

func isHashLess(a, b [32]byte) bool {
	for i := 0; i < 32; i++ {
		if a[i] < b[i] {
			return true
		}
		if a[i] > b[i] {
			return false
		}
	}
	return false
}

func hashMap(t reflect.Type, v reflect.Value) (hash [32]byte, err error) {
	h := sha3pool.Get256()
	defer sha3pool.Put256(h)

	var kvs kvs

	h.Write([]byte("map"))
	for _, key := range v.MapKeys() {
		kv := new(kv)
		kv.k, err = Hash(key)
		if err != nil {
			return hash, err
		}
		kv.v, err = Hash(v.MapIndex(key))
		if err != nil {
			return hash, err
		}
		kvs = append(kvs, kv)
	}
	sort.Sort(kvs)
	for _, kv := range kvs {
		h.Write(kv.k[:])
		h.Write(kv.v[:])
	}
	h.Read(hash[:])
	return hash, nil
}

func hashPtr(t reflect.Type, v reflect.Value) (hash [32]byte, err error) {
	h := sha3pool.Get256()
	defer sha3pool.Put256(h)

	h.Write([]byte("pointer"))
	subhash, err := Hash(v.Elem())
	if err != nil {
		return hash, err
	}
	h.Write(subhash[:])
	h.Read(hash[:])
	return hash, nil
}

func hashSlice(t reflect.Type, v reflect.Value) (hash [32]byte, err error) {
	h := sha3pool.Get256()
	defer sha3pool.Put256(h)

	h.Write([]byte("slice"))
	for i := 0; i < v.Len(); i++ {
		subhash, err := Hash(v.Index(i))
		if err != nil {
			return hash, err
		}
		h.Write(subhash[:])
	}
	h.Read(hash[:])
	return hash, nil
}

func hashString(s string) (hash [32]byte) {
	h := sha3pool.Get256()
	defer sha3pool.Put256(h)

	h.Write([]byte("string"))
	h.Write([]byte(s))
	h.Read(hash[:])
	return hash
}

type (
	field struct {
		name string
		hash [32]byte
	}
	fields []*field
)

func (f *fields) Len() int           { return len(f) }
func (f *fields) Less(i, j int) bool { return f[i].name < f[j].name }
func (f *fields) Swap(i, j int)      { f[i], f[j] = f[j], f[i] }

func hashStruct(t reflect.Type, v reflect.Value) (hash [32]byte, err error) {
	h := sha3pool.Get256()
	defer sha3pool.Put256(h)

	h.Write([]byte("struct"))

	var fields fields

	for i := 0; i < t.NumField(); i++ {
		f := t.Field(i)
		if f.PkgPath != "" {
			// the field is not exported
			continue
		}
		field := new(field)
		field.name = f.Name
		field.hash, err = Hash(v.Field(i))
		if err != nil {
			return hash, err
		}
		fields = append(fields, field)
	}
	sort.Sort(fields) // sort by field name
	for _, field := range fields {
		h.Write([]byte(field.name))
		h.Write(field.hash[:])
	}
	h.Read(hash[:])
	return hash, nil
}
