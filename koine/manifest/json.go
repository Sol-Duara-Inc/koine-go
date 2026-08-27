package manifest

import (
	"bytes"
	"encoding/json"
)

// JSON renders the manifest in the form that rides the guest's `manifest`
// export: two-space indented, one trailing newline, fields in declaration
// order. Struct order is the only order here — there is no map to iterate —
// so the same code always produces the same bytes, which is what makes a
// committed manifest golden a real gate.
//
// encoding/json is used on this path deliberately and only here: a manifest
// is a BUILD-TIME artifact produced by koinegen on a developer's machine,
// never marshalled inside a guest. The guest carries the bytes, not the
// marshaller — which is why nothing in the generated stratum code reaches
// for this package.
func (m Manifest) JSON() ([]byte, error) {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	if err := enc.Encode(m); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}
