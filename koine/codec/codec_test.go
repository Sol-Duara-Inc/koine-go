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
