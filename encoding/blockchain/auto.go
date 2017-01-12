package blockchain

import (
	"fmt"
	"io"
	"reflect"
	"strconv"
	"strings"
)

type BCWriterTo interface {
	BCWriteTo(w io.Writer, serflags uint8) (int, error)
}

var bcWriterToType = reflect.TypeOf((*BCWriterTo)(nil)).Elem()

func Write(w io.Writer, serflags uint8, writeFlags bool, obj interface{}) (n int, err error) {
	if writeFlags {
		n, err = w.Write([]byte{serflags})
		if err != nil {
			return n, err
		}
	}

	if writerTo, ok := obj.(BCWriterTo); ok {
		return writerTo.BCWriteTo(w, serflags)
	}

	t := reflect.TypeOf(obj)
	v := reflect.ValueOf(obj)

	switch t.Kind() {
	case reflect.Ptr:
		// Dereference the pointer and try again.
		return Write(w, serflags, false, v.Elem().Interface())

	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
		return WriteVarint63(w, v.Uint()) // note: counts and outpoint indexes use varint31

	case reflect.Array:
		if t.Elem().Kind() == reflect.Uint8 {
			// fixed-length array of bytes gets written as-is
			l := t.Len()
			s := make([]byte, l, l)
			sv := reflect.ValueOf(s)
			reflect.Copy(sv, v)
			return w.Write(sv.Bytes())
		}
		return n, fmt.Errorf("blockchain.Write cannot handle arrays of %s", t.Elem().Kind())

	case reflect.Slice:
		t2 := t.Elem()
		if t2.Kind() == reflect.Slice && t2.Elem().Kind() == reflect.Uint8 {
			// obj is a [][]byte
			return WriteVarstrList(w, obj.([][]byte))
		}
		// obj is a []SomethingElse. Write a length prefix and then
		// (recursively) the elements of obj one after another.
		l := v.Len()
		n2, err := WriteVarint31(w, uint64(l))
		n += n2
		if err != nil {
			return n, err
		}
		for i := 0; i < l; i++ {
			elt := v.Index(i)
			n2, err := Write(w, serflags, false, elt.Interface())
			n += n2
			if err != nil {
				return n, err
			}
		}
		return n, nil

	case reflect.Struct:
		l := t.NumField()
		var nextField int
		for nextField < l {
			var n2 int
			n2, nextField, err = writeStructFields(w, t, v, serflags, nextField, 0)
			n += n2
			if err != nil {
				return n, err
			}
		}
		return n, nil
	}
	return n, fmt.Errorf("blockchain.Write cannot handle objects of type %s", t.Kind())
}

func writeStructFields(w io.Writer, t reflect.Type, v reflect.Value, serflags uint8, idx, extStr int) (n, nextField int, err error) {
	const tagName = "bc"

	for i := idx; i < t.NumField(); i++ {
		var int31 bool

		tf := t.Field(i)
		if tag, ok := tf.Tag.Lookup(tagName); ok {
			parsedTag, err := parseTag(tag)
			if err != nil {
				return n, 0, err
			}

			int31 = parsedTag.int31
			if parsedTag.extStr > 0 {
				switch extStr {
				case 0:
					// Start a new extensible string.
					n, err := WriteExtensibleString(w, func(w io.Writer) error {
						_, nextField, err = writeStructFields(w, t, v, serflags, i, parsedTag.extStr)
						return err
					})
					// Allow caller to resume from this point.
					return n, nextField, err

				case parsedTag.extStr:
					// Already in an extensible string, proceed normally (below).

				default:
					// Already in an extensible string that has ended. Return
					// and allow the caller to start a new one.
					return n, i, nil
				}
			}
		}
		vf := v.Field(i)
		var intf interface{}
		if int31 {
			intf = varint31(vf.Uint())
		} else {
			intf = vf.Interface()
		}
		n2, err := Write(w, serflags, false, intf)
		n += n2
		if err != nil {
			return n, 0, err
		}
	}
	return n, t.NumField(), nil
}

type parsedTag struct {
	int31  bool
	extStr int
}

func parseTag(tag string) (result parsedTag, err error) {
	const extstrPrefix = "extstr="

	items := strings.Split(tag, ",")
	for _, item := range items {
		if item == "varint31" {
			result.int31 = true
		} else if strings.HasPrefix(item, extstrPrefix) {
			suffix := strings.TrimPrefix(item, extstrPrefix)
			val, err := strconv.Atoi(suffix)
			if err != nil {
				return result, err
			}
			result.extStr = val
		}
	}
	return result, nil
}

// varint31 uses (Read|Write)Varint31 instead of (Read|Write)Varint63.
type varint31 uint64

func (v varint31) BCWriteTo(w io.Writer, _ uint8) (int, error) {
	return WriteVarint31(w, uint64(v))
}
