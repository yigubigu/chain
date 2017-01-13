package blockchain

import (
	"fmt"
	"io"
	"reflect"
	"strconv"
	"strings"
)

type (
	BCWriterTo interface {
		BCWriteTo(w io.Writer, serflags uint8) (int, error)
	}
	BCReaderFrom interface {
		BCReadFrom(io.Reader) (int, error)
	}
)

var (
	bcWriterToType   = reflect.TypeOf((*BCWriterTo)(nil)).Elem()
	bcReaderFromType = reflect.TypeOf((*BCReaderFrom)(nil)).Elem()
)

const tagName = "bc"

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

func (v *varint31) BCReadFrom(r io.Reader) (int, error) {
	val, n, err := ReadVarint31(r)
	if err != nil {
		return n, err
	}
	*v = varint31(val)
	return n, nil
}

func Read(r io.Reader, inp interface{}) (int, error) {
	if readerFrom, ok := inp.(BCReaderFrom); ok {
		return readerFrom.BCReadFrom(r)
	}
	t := reflect.TypeOf(inp)
	if t.Kind() != reflect.Ptr {
		return 0, fmt.Errorf("blockchain.Read called with a non-pointer argument (%s)", t.Kind())
	}
	return readVal(r, reflect.ValueOf(inp).Elem())
}

func readVal(r io.Reader, v reflect.Value) (n int, err error) {
	t := v.Type()

	switch t.Kind() {
	case reflect.Uint8:
		var b [1]byte
		n, err = io.ReadFull(r, b[:])
		if err != nil {
			return n, err
		}
		v.SetUint(uint64(b[0]))
		return n, nil

	case reflect.Uint64:
		val, n, err := ReadVarint63(r)
		if err != nil {
			return n, err
		}
		v.SetUint(val)
		return n, nil

	case reflect.Array:
		if t.Elem().Kind() == reflect.Uint8 {
			// fixed-length array of bytes gets read as-is
			l := t.Len()
			s := make([]byte, l, l)
			n, err := io.ReadFull(r, s[:])
			if err != nil {
				return n, err
			}
			sv := reflect.ValueOf(s)
			reflect.Copy(v, sv)
			return n, nil
		}
		return n, fmt.Errorf("blockchain.Read cannot handle arrays of %s", t.Elem().Kind())

	case reflect.Slice:
		t2 := t.Elem()
		if t2.Kind() == reflect.Slice && t2.Elem().Kind() == reflect.Uint8 {
			// v is a [][]byte
			b, n, err := ReadVarstrList(r)
			if err != nil {
				return n, err
			}
			v.Set(reflect.ValueOf(b))
			return n, nil
		}
		// v is a []SomethingElse. Read a length prefix and then
		// (recursively) the elements of v one after another.
		count, n, err := ReadVarint31(r)
		if err != nil {
			return n, err
		}
		res := reflect.MakeSlice(t2, 0, int(count))
		for i := uint32(0); i < count; i++ {
			elVal := reflect.New(t2)
			n2, err := readVal(r, elVal)
			n += n2
			if err != nil {
				return n, err
			}
			res = reflect.Append(res, elVal)
		}
		v.Set(res)
		return n, err

	case reflect.Struct:
		l := t.NumField()
		var nextField int
		for nextField < l {
			var n2 int
			n2, nextField, err = readStructFields(r, t, v, nextField, 0)
			n += n2
			if err != nil {
				return n, err
			}
		}
		return n, nil
	}

	return n, fmt.Errorf("blockchain.Read cannot handle objects of type %s", t.Kind())
}

func readStructFields(r io.Reader, t reflect.Type, v reflect.Value, idx, extStr int) (n, nextField int, err error) {
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
					// Start reading from a new extensible string.
					n, err := ReadExtensibleString(r, true, func(r io.Reader) error { // xxx "all" argument slated for removal
						_, nextField, err = readStructFields(r, t, v, i, parsedTag.extStr)
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
		if int31 {
			var x varint31
			n2, err := Read(r, &x)
			n += n2
			if err != nil {
				return n, 0, err
			}
			vf.SetUint(uint64(x))
		} else {
			n2, err := readVal(r, vf)
			n += n2
			if err != nil {
				return n, 0, err
			}
		}
	}
	return n, t.NumField(), nil
}
