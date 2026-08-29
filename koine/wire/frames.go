package wire

import (
	"errors"
	"sort"

	"github.com/sol-duara-inc/koine-go/koine"
	"github.com/sol-duara-inc/koine-go/koine/codec"
)

// The frames. EVERY KEY, TYPE AND ORDER BELOW IS THE HOST'S — they are
// koinehost.Delivery, koinehost.ExchangeRequest and koinehost.ExchangeResponse
// rendered without reflection. The host marshals its half with encoding/json,
// so the order here is its struct order and the omitempty rules are its tags;
// a round-trip test in the conformance module compares these bytes against
// the host's own marshaller rather than against this file's opinion.
//
// Payloads — a station's projected facts, a fulfiller's answer — ride as raw
// JSON. The wire moves them without parsing them, because their shape
// belongs to a stratum and the wire has no business knowing a stratum's
// shape. That is also why a delivery may grow keys (conduit-go#184's run
// register) without moving Version: the wire carries whatever it is handed.

// ErrFrame is raised when a frame is structurally unreadable.
var ErrFrame = errors.New("koine/wire: frame is not readable")

// DeliveryFrame is host→guest: the projected facts plus the construction
// context. It is koinehost.Delivery.
//
// Two facts this SDK used to carry have no field of their own here, and are
// carried where the host's shape has room for them rather than invented into
// it: the station name is the host's (it names the station at Load and
// tracks it per invocation), and the anchor rides in Context under
// ContextAnchor.
type DeliveryFrame struct {
	Version   int               // the contract version; Version above
	Event     []byte            // projected JSON, already cut to the lineage
	EventType string            // the event type delivered
	Subject   string            // omitted when empty
	RunID     string            //
	ChainID   string            // where the station stands in the flow
	Actor     string            // whose authority it carries; omitted when empty
	Context   map[string]string // omitted when empty
}

// ContextAnchor is the Context key the anchor name rides under. The host's
// Delivery has no anchor field; Context is a free map, so the anchor lives
// there by convention rather than by a field this SDK invented on a shape it
// does not own.
const ContextAnchor = "anchor"

// Anchor is the anchor name the arrival filled, or empty.
func (f DeliveryFrame) Anchor() string { return f.Context[ContextAnchor] }

// EncodeFields writes the frame's keys in the host's struct order, honouring
// the host's omitempty tags.
func (f DeliveryFrame) EncodeFields(w *codec.Writer) {
	w.Key("version")
	w.Int(f.Version)
	w.Key("event")
	writeRaw(w, f.Event)
	w.Key("eventType")
	w.String(f.EventType)
	if f.Subject != "" {
		w.Key("subject")
		w.String(f.Subject)
	}
	w.Key("runId")
	w.String(f.RunID)
	w.Key("chainId")
	w.String(f.ChainID)
	if f.Actor != "" {
		w.Key("actor")
		w.String(f.Actor)
	}
	if len(f.Context) > 0 {
		w.Key("context")
		w.BeginObject()
		for _, k := range sortedKeys(f.Context) {
			w.Key(k)
			w.String(f.Context[k])
		}
		w.EndObject()
	}
}

// MarshalJSON renders the frame without reflection.
func (f DeliveryFrame) MarshalJSON() ([]byte, error) { return marshal(f) }

// DecodeDelivery reads a delivery frame and gates its version.
func DecodeDelivery(data []byte) (DeliveryFrame, error) {
	var f DeliveryFrame
	err := codec.DecodeObject(data, func(key string, r *codec.Reader) (bool, error) {
		switch key {
		case "version":
			v, err := r.Int()
			f.Version = v
			return true, err
		case "event":
			return readRaw(r, &f.Event)
		case "eventType":
			return readString(r, &f.EventType)
		case "subject":
			return readString(r, &f.Subject)
		case "runId":
			return readString(r, &f.RunID)
		case "chainId":
			return readString(r, &f.ChainID)
		case "actor":
			return readString(r, &f.Actor)
		case "context":
			f.Context = map[string]string{}
			return true, r.Object(func(k string, r *codec.Reader) (bool, error) {
				v, err := r.String()
				if err != nil {
					return true, err
				}
				f.Context[k] = v
				return true, nil
			})
		}
		return false, nil
	})
	if err != nil {
		return f, err
	}
	return f, Accepts(f.Version)
}

// YieldFrame is guest→host: ONE utterance, and the whole of the guest's emit
// path.
//
// The bytes the host receives ARE the domain object. The host reads the type
// out of them and stores the very same bytes as the payload — so a wrapper
// around the object would be stored INSTEAD of the object. This frame
// therefore writes the type key and then the object's own keys, flat,
// beside it. A generated stratum may not declare a wire key named "type";
// koinegen refuses that collision at load, which is what makes this safe.
type YieldFrame struct {
	Type string    // the event type spoken
	Body Speakable // the domain object itself
}

// EncodeFields writes the type and then the object's own keys, flat.
func (f YieldFrame) EncodeFields(w *codec.Writer) {
	w.Key(YieldTypeKey)
	w.String(f.Type)
	if f.Body != nil {
		f.Body.EncodeFields(w)
	}
}

// YieldTypeKey is the key the host reads the event type out of. It is also
// the key no stratum may declare — see koinegen's identifier judgement.
const YieldTypeKey = "type"

// MarshalJSON renders the frame without reflection.
func (f YieldFrame) MarshalJSON() ([]byte, error) { return marshal(f) }

// ExchangeFrame is guest→host: an intent, uttered at a seat. It is
// koinehost.ExchangeRequest.
//
// The host's shape has no seat: it routes on Type, which is the intent's
// name — "history.last" — and that is what a broker's handlers are keyed on.
// The seat still reaches the deployment, through the manifest, which is
// where an unfilled seat is refused by name before any of this runs (§7.3).
type ExchangeFrame struct {
	Type    string            // the intent spoken — "history.last"
	Intent  []byte            // omitted when empty
	Filter  map[string]string // the intent's arguments; omitted when empty
	Outcome string            // omitted when empty
}

// ArgOutcome is the argument name the host gives a field of its own. The
// resolved idiom asks the record for a terminal set by outcome often enough
// that koinehost.ExchangeRequest carries Outcome beside Filter; an argument
// by that name therefore rides in the field built for it, and nowhere else,
// so one fact lives in one place.
const ArgOutcome = "outcome"

// NewExchangeFrame renders a spoken exchange into the host's request shape.
func NewExchangeFrame(ex koine.Exchange) ExchangeFrame {
	f := ExchangeFrame{Type: ex.Name}
	for _, a := range ex.Args {
		if a.Name == ArgOutcome {
			f.Outcome = a.Value
			continue
		}
		if f.Filter == nil {
			f.Filter = map[string]string{}
		}
		f.Filter[a.Name] = a.Value
	}
	return f
}

// EncodeFields writes the frame's keys in the host's struct order.
func (f ExchangeFrame) EncodeFields(w *codec.Writer) {
	w.Key("type")
	w.String(f.Type)
	if len(f.Intent) > 0 {
		w.Key("intent")
		writeRaw(w, f.Intent)
	}
	if len(f.Filter) > 0 {
		w.Key("filter")
		w.BeginObject()
		for _, k := range sortedKeys(f.Filter) {
			w.Key(k)
			w.String(f.Filter[k])
		}
		w.EndObject()
	}
	if f.Outcome != "" {
		w.Key("outcome")
		w.String(f.Outcome)
	}
}

// MarshalJSON renders the frame without reflection.
func (f ExchangeFrame) MarshalJSON() ([]byte, error) { return marshal(f) }

// DecodeExchange reads an exchange frame. It carries no version key — the
// host's ExchangeRequest has none — so there is nothing to gate here.
func DecodeExchange(data []byte) (ExchangeFrame, error) {
	var f ExchangeFrame
	err := codec.DecodeObject(data, func(key string, r *codec.Reader) (bool, error) {
		switch key {
		case "type":
			return readString(r, &f.Type)
		case "intent":
			return readRaw(r, &f.Intent)
		case "filter":
			f.Filter = map[string]string{}
			return true, r.Object(func(k string, r *codec.Reader) (bool, error) {
				v, err := r.String()
				if err != nil {
					return true, err
				}
				f.Filter[k] = v
				return true, nil
			})
		case "outcome":
			return readString(r, &f.Outcome)
		}
		return false, nil
	})
	return f, err
}

// AnswerFrame is host→guest: the answer to an exchange. It is
// koinehost.ExchangeResponse.
//
// There is no "pending" here, and that is not an omission. The host's
// value_poll WAITS — it blocks on the exchange's own completion — so by the
// time these bytes exist the future has already gone one way or the other.
// E-C's "Value() waits until the exchange is filled or breached" is
// satisfied below the line, which is exactly where the ruling left the
// mechanism.
type AnswerFrame struct {
	Status   int    //
	Value    []byte // omitted when empty
	Error    string // the typed outcome variant; omitted when empty
	Breached bool   // omitted when false
}

// EncodeFields writes the frame's keys in the host's struct order.
func (f AnswerFrame) EncodeFields(w *codec.Writer) {
	w.Key("status")
	w.Int(f.Status)
	if len(f.Value) > 0 {
		w.Key("value")
		writeRaw(w, f.Value)
	}
	if f.Error != "" {
		w.Key("error")
		w.String(f.Error)
	}
	if f.Breached {
		w.Key("breached")
		w.Bool(true)
	}
}

// MarshalJSON renders the frame without reflection.
func (f AnswerFrame) MarshalJSON() ([]byte, error) { return marshal(f) }

// DecodeAnswer reads an answer frame.
func DecodeAnswer(data []byte) (AnswerFrame, error) {
	var f AnswerFrame
	err := codec.DecodeObject(data, func(key string, r *codec.Reader) (bool, error) {
		switch key {
		case "status":
			v, err := r.Int()
			f.Status = v
			return true, err
		case "value":
			return readRaw(r, &f.Value)
		case "error":
			return readString(r, &f.Error)
		case "breached":
			v, err := r.Bool()
			f.Breached = v
			return true, err
		}
		return false, nil
	})
	return f, err
}

// Breach reports whether the answer is the future having gone the other way.
// A breach flag, or an error the host named, are the same fact said twice;
// either is enough.
func (f AnswerFrame) Breach() bool { return f.Breached || f.Error != "" }

// frame is what every frame in this package can do.
type frame interface{ EncodeFields(*codec.Writer) }

func marshal(f frame) ([]byte, error) {
	var w codec.Writer
	w.BeginObject()
	f.EncodeFields(&w)
	w.EndObject()
	return w.Bytes(), nil
}

// writeRaw splices an already-encoded payload, or null where there is none —
// which is what encoding/json writes for a nil json.RawMessage, and what the
// host will therefore both send and expect.
func writeRaw(w *codec.Writer, body []byte) {
	if len(body) == 0 {
		w.Null()
		return
	}
	w.Raw(body)
}

func readString(r *codec.Reader, into *string) (bool, error) {
	v, err := r.String()
	if err != nil {
		return true, err
	}
	*into = v
	return true, nil
}

func readRaw(r *codec.Reader, into *[]byte) (bool, error) {
	v, err := r.Raw()
	if err != nil {
		return true, err
	}
	if string(v) == "null" {
		return true, nil
	}
	*into = v
	return true, nil
}

// sortedKeys orders a map the way encoding/json does, so the guest's bytes
// and the host's bytes are the same bytes.
func sortedKeys(m map[string]string) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
