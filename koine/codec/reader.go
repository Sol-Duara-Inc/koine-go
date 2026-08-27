package codec

import (
	"errors"
	"strconv"
	"strings"
	"unicode/utf16"
	"unicode/utf8"
)

// Reader scans a JSON document one value at a time. Generated DecodeField
// methods hold it while they read the value that belongs to a key they
// named; every key they did not name is skipped, which is how a stratum
// tolerates the keys of the strata below it without ever seeing them as
// fields.
type Reader struct {
	data []byte
	pos  int
}

// NewReader reads the given document.
func NewReader(data []byte) *Reader { return &Reader{data: data} }

// FieldFunc is the per-member callback DecodeObject drives. It reports
// whether it consumed the value; an unconsumed value is skipped.
type FieldFunc func(key string, r *Reader) (bool, error)

// DecodeObject reads a whole JSON object, handing each member to field in
// document order. A null document decodes to no members at all — the zero
// value stands, which is what an absent projection means.
func DecodeObject(data []byte, field FieldFunc) error {
	r := NewReader(data)
	r.space()
	if r.match("null") {
		return r.atEnd()
	}
	if err := r.Object(field); err != nil {
		return err
	}
	return r.atEnd()
}

// Object reads one object value from the current position.
func (r *Reader) Object(field FieldFunc) error {
	if err := r.expect('{'); err != nil {
		return err
	}
	r.space()
	if r.peek() == '}' {
		r.pos++
		return nil
	}
	for {
		r.space()
		key, err := r.String()
		if err != nil {
			return err
		}
		r.space()
		if err := r.expect(':'); err != nil {
			return err
		}
		r.space()
		handled, err := field(key, r)
		if err != nil {
			return err
		}
		if !handled {
			if err := r.Skip(); err != nil {
				return err
			}
		}
		r.space()
		switch r.peek() {
		case ',':
			r.pos++
		case '}':
			r.pos++
			return nil
		default:
			return r.fail("expected ',' or '}'")
		}
	}
}

// String reads a JSON string.
func (r *Reader) String() (string, error) {
	if err := r.expect('"'); err != nil {
		return "", err
	}
	start := r.pos
	for r.pos < len(r.data) {
		c := r.data[r.pos]
		switch {
		case c == '"':
			s := string(r.data[start:r.pos])
			r.pos++
			return s, nil
		case c == '\\':
			return r.stringEscaped(start)
		case c < 0x20:
			return "", r.fail("control character in string")
		default:
			r.pos++
		}
	}
	return "", r.fail("unterminated string")
}

// Int reads a JSON number that names a whole number.
func (r *Reader) Int() (int, error) {
	lit, err := r.number()
	if err != nil {
		return 0, err
	}
	n, err := strconv.Atoi(lit)
	if err != nil {
		return 0, r.fail("expected a whole number, got " + lit)
	}
	return n, nil
}

// Bool reads true or false.
func (r *Reader) Bool() (bool, error) {
	r.space()
	switch {
	case r.match("true"):
		return true, nil
	case r.match("false"):
		return false, nil
	}
	return false, r.fail("expected true or false")
}

// Skip discards the value at the current position, whatever its shape.
func (r *Reader) Skip() error {
	r.space()
	switch c := r.peek(); {
	case c == '"':
		_, err := r.String()
		return err
	case c == '{' || c == '[':
		return r.skipContainer()
	case r.match("true") || r.match("false") || r.match("null"):
		return nil
	case c == '-' || (c >= '0' && c <= '9'):
		_, err := r.number()
		return err
	}
	return r.fail("expected a value")
}

func (r *Reader) skipContainer() error {
	depth := 0
	for r.pos < len(r.data) {
		switch r.data[r.pos] {
		case '"':
			if _, err := r.String(); err != nil {
				return err
			}
			continue
		case '{', '[':
			depth++
		case '}', ']':
			depth--
		}
		r.pos++
		if depth == 0 {
			return nil
		}
	}
	return r.fail("unterminated object or array")
}

func (r *Reader) stringEscaped(start int) (string, error) {
	var b strings.Builder
	b.Write(r.data[start:r.pos])
	for r.pos < len(r.data) {
		c := r.data[r.pos]
		switch {
		case c == '"':
			r.pos++
			return b.String(), nil
		case c < 0x20:
			return "", r.fail("control character in string")
		case c != '\\':
			b.WriteByte(c)
			r.pos++
			continue
		}
		r.pos++
		if r.pos >= len(r.data) {
			return "", r.fail("unterminated escape")
		}
		e := r.data[r.pos]
		r.pos++
		switch e {
		case '"', '\\', '/':
			b.WriteByte(e)
		case 'b':
			b.WriteByte('\b')
		case 'f':
			b.WriteByte('\f')
		case 'n':
			b.WriteByte('\n')
		case 'r':
			b.WriteByte('\r')
		case 't':
			b.WriteByte('\t')
		case 'u':
			rn, err := r.unicodeEscape()
			if err != nil {
				return "", err
			}
			b.WriteRune(rn)
		default:
			return "", r.fail("unknown escape")
		}
	}
	return "", r.fail("unterminated string")
}

func (r *Reader) unicodeEscape() (rune, error) {
	first, err := r.hex4()
	if err != nil {
		return 0, err
	}
	if !utf16.IsSurrogate(rune(first)) {
		return rune(first), nil
	}
	if r.pos+1 < len(r.data) && r.data[r.pos] == '\\' && r.data[r.pos+1] == 'u' {
		save := r.pos
		r.pos += 2
		second, err := r.hex4()
		if err != nil {
			return 0, err
		}
		if rn := utf16.DecodeRune(rune(first), rune(second)); rn != utf8.RuneError {
			return rn, nil
		}
		r.pos = save
	}
	return utf8.RuneError, nil
}

func (r *Reader) hex4() (uint32, error) {
	if r.pos+4 > len(r.data) {
		return 0, r.fail("short unicode escape")
	}
	v, err := strconv.ParseUint(string(r.data[r.pos:r.pos+4]), 16, 32)
	if err != nil {
		return 0, r.fail("bad unicode escape")
	}
	r.pos += 4
	return uint32(v), nil
}

func (r *Reader) number() (string, error) {
	r.space()
	start := r.pos
	if r.peek() == '-' {
		r.pos++
	}
	for r.pos < len(r.data) {
		c := r.data[r.pos]
		if (c >= '0' && c <= '9') || c == '.' || c == 'e' || c == 'E' || c == '+' || c == '-' {
			r.pos++
			continue
		}
		break
	}
	if r.pos == start {
		return "", r.fail("expected a number")
	}
	return string(r.data[start:r.pos]), nil
}

func (r *Reader) match(lit string) bool {
	if r.pos+len(lit) <= len(r.data) && string(r.data[r.pos:r.pos+len(lit)]) == lit {
		r.pos += len(lit)
		return true
	}
	return false
}

func (r *Reader) expect(c byte) error {
	r.space()
	if r.peek() != c {
		return r.fail("expected " + strconv.QuoteRune(rune(c)))
	}
	r.pos++
	return nil
}

func (r *Reader) peek() byte {
	if r.pos >= len(r.data) {
		return 0
	}
	return r.data[r.pos]
}

func (r *Reader) space() {
	for r.pos < len(r.data) {
		switch r.data[r.pos] {
		case ' ', '\t', '\n', '\r':
			r.pos++
		default:
			return
		}
	}
}

func (r *Reader) atEnd() error {
	r.space()
	if r.pos != len(r.data) {
		return r.fail("trailing bytes after the document")
	}
	return nil
}

func (r *Reader) fail(why string) error {
	return errors.New("koine/codec: " + why + " at byte " + strconv.Itoa(r.pos))
}
