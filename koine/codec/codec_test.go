package codec_test

import (
	"strings"
	"testing"

	"github.com/sol-duara-inc/koine-go/koine/codec"
)

// TestCodec_WriterSeparatesWithoutBeingTold pins the one thing generated
// code must not have to think about: commas. A generator that had to track
// separators would put the tracking in every emitted line.
func TestCodec_WriterSeparatesWithoutBeingTold(t *testing.T) {
	var w codec.Writer
	w.BeginObject()
	w.Key("subject")
	w.String("payments-api")
	w.Key("queueWaitMs")
	w.Int(42)
	w.Key("pciScope")
	w.Bool(true)
	w.Key("note")
	w.Null()
	w.Key("nested")
	w.BeginObject()
	w.Key("a")
	w.String("b")
	w.EndObject()
	w.Key("list")
	w.BeginArray()
	w.Int(1)
	w.Int(2)
	w.EndArray()
	w.EndObject()

	const want = `{"subject":"payments-api","queueWaitMs":42,"pciScope":true,"note":null,"nested":{"a":"b"},"list":[1,2]}`
	if got := string(w.Bytes()); got != want {
		t.Fatalf("wrote\n  %s\nwant\n  %s", got, want)
	}
}

// TestCodec_WriterEscapesWhatMustBeEscaped keeps a domain string from
// forging structure on its way out.
func TestCodec_WriterEscapesWhatMustBeEscaped(t *testing.T) {
	var w codec.Writer
	w.String("he said \"hi\"\n\tand \\ left\x01")
	const want = `"he said \"hi\"\n\tand \\ left\u0001"`
	if got := string(w.Bytes()); got != want {
		t.Fatalf("wrote %s want %s", got, want)
	}
}

// TestCodec_DecodeObjectSkipsWhatItWasNotAskedFor is the projection wall at
// the byte level: a stratum names its own keys and everything else is
// skipped whole, however deeply nested.
func TestCodec_DecodeObjectSkipsWhatItWasNotAskedFor(t *testing.T) {
	const doc = `{"kept":"yes","nested":{"deep":[1,{"x":"y"}]},"n":-17,"b":false,"gone":null,"esc":"aé😀b"}`
	var kept, esc string
	var n int
	var b bool
	var seen []string
	err := codec.DecodeObject([]byte(doc), func(key string, r *codec.Reader) (bool, error) {
		seen = append(seen, key)
		switch key {
		case "kept":
			v, err := r.String()
			kept = v
			return true, err
		case "n":
			v, err := r.Int()
			n = v
			return true, err
		case "b":
			v, err := r.Bool()
			b = v
			return true, err
		case "esc":
			v, err := r.String()
			esc = v
			return true, err
		}
		return false, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if kept != "yes" || n != -17 || b {
		t.Fatalf("kept=%q n=%d b=%v", kept, n, b)
	}
	if esc != "aé\U0001f600b" {
		t.Fatalf("escapes decoded to %q", esc)
	}
	if got := strings.Join(seen, ","); got != "kept,nested,n,b,gone,esc" {
		t.Fatalf("keys arrived as %q", got)
	}
}

// TestCodec_NullDecodesToTheZeroValue pins what an absent projection means:
// no members at all, and the zero value stands.
func TestCodec_NullDecodesToTheZeroValue(t *testing.T) {
	called := false
	err := codec.DecodeObject([]byte("null"), func(string, *codec.Reader) (bool, error) {
		called = true
		return false, nil
	})
	if err != nil || called {
		t.Fatalf("null produced err=%v called=%v", err, called)
	}
	if err := codec.DecodeObject([]byte("{}"), func(key string, r *codec.Reader) (bool, error) {
		t.Errorf("an empty object produced the key %q", key)
		return false, nil
	}); err != nil {
		t.Fatal(err)
	}
}

// TestCodec_MalformedInputIsRefusedByName keeps the codec on the same
// posture as everything else here: refuse, and say where.
func TestCodec_MalformedInputIsRefusedByName(t *testing.T) {
	for _, doc := range []string{
		`{"a":}`,
		`{"a":1`,
		`{"a" 1}`,
		`{"a":"unterminated}`,
		`{"a":1} trailing`,
		`[1,2]`,
		`{"a":1.5}`,
	} {
		err := codec.DecodeObject([]byte(doc), func(key string, r *codec.Reader) (bool, error) {
			if key == "a" {
				_, err := r.Int()
				return true, err
			}
			return false, nil
		})
		if err == nil {
			t.Errorf("%s was accepted", doc)
			continue
		}
		if !strings.HasPrefix(err.Error(), "koine/codec: ") {
			t.Errorf("%s failed anonymously: %v", doc, err)
		}
	}
}

// TestCodec_RawCarriesAValueWithoutReadingIt pins how a frame moves a
// payload it does not own. The wire carries a station's projected facts and
// a fulfiller's answer across the boundary without parsing them, because
// their shape belongs to a stratum and the wire has no business knowing a
// stratum's shape.
func TestCodec_RawCarriesAValueWithoutReadingIt(t *testing.T) {
	const doc = `{"facts":{"a":[1,{"b":"}"},null],"n":-1.5e3},"after":"kept"}`
	var facts []byte
	var after string
	err := codec.DecodeObject([]byte(doc), func(key string, r *codec.Reader) (bool, error) {
		switch key {
		case "facts":
			v, err := r.Raw()
			facts = v
			return true, err
		case "after":
			v, err := r.String()
			after = v
			return true, err
		}
		return false, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	const want = `{"a":[1,{"b":"}"},null],"n":-1.5e3}`
	if string(facts) != want {
		t.Fatalf("captured\n  %s\nwant\n  %s", facts, want)
	}
	if after != "kept" {
		t.Fatalf("the reader lost its place after a raw capture: %q", after)
	}

	// Raw copies. A caller that keeps the bytes keeps them, whatever
	// happens to the document afterwards.
	source := []byte(`{"x":{"deep":1}}`)
	var held []byte
	if err := codec.DecodeObject(source, func(key string, r *codec.Reader) (bool, error) {
		v, err := r.Raw()
		held = v
		return true, err
	}); err != nil {
		t.Fatal(err)
	}
	for i := range source {
		source[i] = ' '
	}
	if string(held) != `{"deep":1}` {
		t.Fatalf("a raw capture aliased its source: %q", held)
	}
}

// TestCodec_Uint64SurvivesItsWholeRange pins the number a token rides on.
// The wire never sends a number it has to round.
func TestCodec_Uint64SurvivesItsWholeRange(t *testing.T) {
	for _, v := range []uint64{0, 1, 1 << 53, 1<<64 - 1} {
		var w codec.Writer
		w.BeginObject()
		w.Key("token")
		w.Uint64(v)
		w.EndObject()

		var got uint64
		if err := codec.DecodeObject(w.Bytes(), func(key string, r *codec.Reader) (bool, error) {
			n, err := r.Uint64()
			got = n
			return true, err
		}); err != nil {
			t.Fatalf("%d: %v", v, err)
		}
		if got != v {
			t.Errorf("%d round-tripped to %d", v, got)
		}
	}
	// A negative number is not a token, and saying so beats truncating.
	err := codec.DecodeObject([]byte(`{"token":-1}`), func(key string, r *codec.Reader) (bool, error) {
		_, err := r.Uint64()
		return true, err
	})
	if err == nil {
		t.Error("a negative token was read")
	}
}

// TestCodec_ArrayReadsElementsInOrder pins the one container the frames
// need: an exchange's arguments, in the order the author wrote them.
func TestCodec_ArrayReadsElementsInOrder(t *testing.T) {
	var got []string
	err := codec.DecodeObject([]byte(`{"args":["a","b","c"],"empty":[]}`), func(key string, r *codec.Reader) (bool, error) {
		if key != "args" && key != "empty" {
			return false, nil
		}
		return true, r.Array(func(r *codec.Reader) error {
			v, err := r.String()
			if err != nil {
				return err
			}
			got = append(got, v)
			return nil
		})
	})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Join(got, "") != "abc" {
		t.Fatalf("elements arrived as %v", got)
	}

	// A malformed array is refused by name like everything else here.
	err = codec.DecodeObject([]byte(`{"args":["a" "b"]}`), func(key string, r *codec.Reader) (bool, error) {
		return true, r.Array(func(r *codec.Reader) error { _, err := r.String(); return err })
	})
	if err == nil || !strings.HasPrefix(err.Error(), "koine/codec: ") {
		t.Fatalf("a malformed array was read: %v", err)
	}
}
