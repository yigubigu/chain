package blockchain

import (
	"fmt"
	"io"
	"reflect"
)

type Writer interface {
	Write(w io.Writer, serflags uint8) (int, error)
}

var writerType = reflect.TypeOf((*Writer)(nil)).Elem()

func Write(w io.Writer, serflags uint8, obj interface{}) (int, error) {
	t := reflect.TypeOf(obj)

	if t.Implements(writerType) {
		writer := obj.(Writer)
		return writer.Write(w, serflags)
	}

	v := reflect.ValueOf(obj)
	switch t.Kind() {
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
		return WriteVarint63(w, v.Uint()) // xxx counts and outpoint indexes use varint31

	case reflect.Slice:
		t2 := t.Elem()
		if t2.Kind() == reflect.Slice && t2.Elem().Kind() == reflect.Uint8 {
			// obj is a [][]byte
			return WriteVarstrList(w, obj.([][]byte))
		}
		// obj is a []SomethingElse. Write a length prefix and then
		// (recursively) the elements of obj one after another.
		l := v.Len()
		n, err := WriteVarint31(w, uint64(l))
		if err != nil {
			return n, err
		}
		for i := 0; i < l; i++ {
			elt := v.Index(i)
			n2, err := Write(w, serflags, elt.Interface())
			n += n2
			if err != nil {
				return n, err
			}
		}
		return n, nil

	case reflect.Struct:
		// xxx
	}
	return 0, fmt.Errorf("blockchain.Write cannot handle objects of type %s", t.Kind())
}
