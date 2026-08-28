package codec

import "strconv"

const hexDigits = "0123456789abcdef"

// Writer builds a JSON document from named parts. Generated EncodeFields
// methods call it in declaration order, so output is byte-stable by
// construction: there is no map to iterate and no order to lose.
type Writer struct {
	buf        []byte
	stack      []bool // per open object/array: has a member been written yet
	pendingKey bool   // the next value fills a member name already written
}

// BeginObject opens an object.
func (w *Writer) BeginObject() {
	w.punct('{')
	w.stack = append(w.stack, false)
}

// EndObject closes the object opened by the matching BeginObject.
func (w *Writer) EndObject() {
	w.pop()
	w.buf = append(w.buf, '}')
}

// BeginArray opens an array.
func (w *Writer) BeginArray() {
	w.punct('[')
	w.stack = append(w.stack, false)
}

// EndArray closes the array opened by the matching BeginArray.
func (w *Writer) EndArray() {
	w.pop()
	w.buf = append(w.buf, ']')
}

// Key writes a member name and its colon, with the separating comma when one
// is owed.
func (w *Writer) Key(name string) {
	w.sep()
	w.writeString(name)
	w.buf = append(w.buf, ':')
}

// String writes a JSON string value.
func (w *Writer) String(s string) {
	w.valueSep()
	w.writeString(s)
}

// Int writes a JSON number.
func (w *Writer) Int(i int) {
	w.valueSep()
	w.buf = strconv.AppendInt(w.buf, int64(i), 10)
}

// Uint64 writes an unsigned JSON number. Tokens and counts ride this way;
// the wire never sends a number it has to round.
func (w *Writer) Uint64(v uint64) {
	w.valueSep()
	w.buf = strconv.AppendUint(w.buf, v, 10)
}

// Bool writes true or false.
func (w *Writer) Bool(b bool) {
	w.valueSep()
	w.buf = strconv.AppendBool(w.buf, b)
}

// Null writes null.
func (w *Writer) Null() {
	w.valueSep()
	w.buf = append(w.buf, 'n', 'u', 'l', 'l')
}

// Raw writes an already-encoded fragment verbatim. It is the seam a nested
// generated type is spliced in through; nothing validates the fragment,
// because the only caller is code this repository generated.
func (w *Writer) Raw(b []byte) {
	w.valueSep()
	w.buf = append(w.buf, b...)
}

// Bytes returns the document written so far. The Writer keeps ownership; a
// caller that retains the slice must copy it.
func (w *Writer) Bytes() []byte { return w.buf }

// Reset empties the Writer for reuse.
func (w *Writer) Reset() {
	w.buf = w.buf[:0]
	w.stack = w.stack[:0]
}

// punct opens a container, taking the separator owed to the container it
// nests in.
func (w *Writer) punct(c byte) {
	w.valueSep()
	w.buf = append(w.buf, c)
}

// valueSep writes the comma owed before an array element or a nested
// container. A value that follows a Key owes nothing — Key already marked
// the member written.
func (w *Writer) valueSep() {
	if n := len(w.stack); n > 0 && !w.pendingKey {
		if w.stack[n-1] {
			w.buf = append(w.buf, ',')
		}
		w.stack[n-1] = true
	}
	w.pendingKey = false
}

// sep writes the comma owed before an object member name.
func (w *Writer) sep() {
	if n := len(w.stack); n > 0 {
		if w.stack[n-1] {
			w.buf = append(w.buf, ',')
		}
		w.stack[n-1] = true
	}
	w.pendingKey = true
}

func (w *Writer) pop() {
	if n := len(w.stack); n > 0 {
		w.stack = w.stack[:n-1]
	}
	w.pendingKey = false
}

func (w *Writer) writeString(s string) {
	w.buf = append(w.buf, '"')
	start := 0
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c >= 0x20 && c != '"' && c != '\\' {
			continue
		}
		w.buf = append(w.buf, s[start:i]...)
		switch c {
		case '"':
			w.buf = append(w.buf, '\\', '"')
		case '\\':
			w.buf = append(w.buf, '\\', '\\')
		case '\n':
			w.buf = append(w.buf, '\\', 'n')
		case '\r':
			w.buf = append(w.buf, '\\', 'r')
		case '\t':
			w.buf = append(w.buf, '\\', 't')
		default:
			w.buf = append(w.buf, '\\', 'u', '0', '0', hexDigits[c>>4], hexDigits[c&0xf])
		}
		start = i + 1
	}
	w.buf = append(w.buf, s[start:]...)
	w.buf = append(w.buf, '"')
}
