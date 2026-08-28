package wire

import (
	"errors"

	"github.com/sol-duara-inc/koine-go/koine"
	"github.com/sol-duara-inc/koine-go/koine/codec"
)

// The frames. Every one of them declares the version it was written under as
// its first key, so a reader can refuse before it has read anything it would
// have to unlearn. Every one of them names its keys in source: nothing here
// reflects, and the guest target stays open (A6).
//
// Payloads — a station's projected facts, a fulfiller's answer — ride as raw
// JSON. The wire moves them without parsing them, because their shape
// belongs to a stratum and the wire has no business knowing a stratum's
// shape. That is also what lets a delivery grow keys while K2 is in flight
// without moving Version: the wire carries whatever it is handed.

// The states an exchange can be in when polled.
const (
	StatePending  = "pending"
	StateReceived = "received"
	StateFilled   = "filled"
	StateBreached = "breached"
)

// ErrFrame is raised when a frame is structurally unreadable.
var ErrFrame = errors.New("koine/wire: frame is not readable")

// DeliveryFrame is host→guest: the projected facts plus the construction
// context (§8). The facts are already cut to the station's lineage before
// the guest sees a byte — the host projects, and the stratum type would not
// hold a foreign key even if one arrived. Two walls, one semantic.
type DeliveryFrame struct {
	Wire    string // the contract version this frame was written under
	Station string // the station this guest is being asked to resolve
	Chain   string // where the station stands in the flow
	Actor   string // whose authority it carries, minted by the host
	Anchor  string // the anchor name the arrival filled, when one was named
	Type    string // the event type delivered
	Facts   []byte // projected JSON, already cut to the lineage
}

// EncodeFields writes the frame's keys in declaration order.
func (f DeliveryFrame) EncodeFields(w *codec.Writer) {
	w.Key("wire")
	w.String(f.Wire)
	w.Key("station")
	w.String(f.Station)
	w.Key("chain")
	w.String(f.Chain)
	w.Key("actor")
	w.String(f.Actor)
	w.Key("anchor")
	w.String(f.Anchor)
	w.Key("type")
	w.String(f.Type)
	w.Key("facts")
	writeRaw(w, f.Facts)
}

// MarshalJSON renders the frame without reflection.
func (f DeliveryFrame) MarshalJSON() ([]byte, error) { return marshal(f) }

// DecodeDelivery reads a delivery frame and gates its version.
func DecodeDelivery(data []byte) (DeliveryFrame, error) {
	var f DeliveryFrame
	err := codec.DecodeObject(data, func(key string, r *codec.Reader) (bool, error) {
		switch key {
		case "wire":
			return readString(r, &f.Wire)
		case "station":
			return readString(r, &f.Station)
		case "chain":
			return readString(r, &f.Chain)
		case "actor":
			return readString(r, &f.Actor)
		case "anchor":
			return readString(r, &f.Anchor)
		case "type":
			return readString(r, &f.Type)
		case "facts":
			return readRaw(r, &f.Facts)
		}
		return false, nil
	})
	if err != nil {
		return f, err
	}
	return f, Accepts(f.Wire)
}

// YieldFrame is guest→host: one utterance, named and rendered. Emitting is
// the storage action, and this frame is the whole of the guest's emit path.
type YieldFrame struct {
	Wire string // the contract version
	Type string // the event type spoken
	Body []byte // the domain object's JSON
}

// EncodeFields writes the frame's keys in declaration order.
func (f YieldFrame) EncodeFields(w *codec.Writer) {
	w.Key("wire")
	w.String(f.Wire)
	w.Key("type")
	w.String(f.Type)
	w.Key("body")
	writeRaw(w, f.Body)
}

// MarshalJSON renders the frame without reflection.
func (f YieldFrame) MarshalJSON() ([]byte, error) { return marshal(f) }

// DecodeYield reads a yield frame and gates its version.
func DecodeYield(data []byte) (YieldFrame, error) {
	var f YieldFrame
	err := codec.DecodeObject(data, func(key string, r *codec.Reader) (bool, error) {
		switch key {
		case "wire":
			return readString(r, &f.Wire)
		case "type":
			return readString(r, &f.Type)
		case "body":
			return readRaw(r, &f.Body)
		}
		return false, nil
	})
	if err != nil {
		return f, err
	}
	return f, Accepts(f.Wire)
}

// ExchangeFrame is guest→host: an intent, uttered at a seat. The guest names
// the seat and the intent; it never names a tool and it never names an
// address, because the operator owns the address and the deployment binds
// the client.
type ExchangeFrame struct {
	Wire string
	Seat string
	Name string
	Args []koine.Arg
}

// EncodeFields writes the frame's keys in declaration order.
func (f ExchangeFrame) EncodeFields(w *codec.Writer) {
	w.Key("wire")
	w.String(f.Wire)
	w.Key("seat")
	w.String(f.Seat)
	w.Key("name")
	w.String(f.Name)
	w.Key("args")
	w.BeginArray()
	for _, a := range f.Args {
		w.BeginObject()
		w.Key("name")
		w.String(a.Name)
		w.Key("value")
		w.String(a.Value)
		w.EndObject()
	}
	w.EndArray()
}

// MarshalJSON renders the frame without reflection.
func (f ExchangeFrame) MarshalJSON() ([]byte, error) { return marshal(f) }

// DecodeExchange reads an exchange frame and gates its version.
func DecodeExchange(data []byte) (ExchangeFrame, error) {
	var f ExchangeFrame
	err := codec.DecodeObject(data, func(key string, r *codec.Reader) (bool, error) {
		switch key {
		case "wire":
			return readString(r, &f.Wire)
		case "seat":
			return readString(r, &f.Seat)
		case "name":
			return readString(r, &f.Name)
		case "args":
			return true, r.Array(func(r *codec.Reader) error {
				var a koine.Arg
				err := r.Object(func(key string, r *codec.Reader) (bool, error) {
					switch key {
					case "name":
						return readString(r, &a.Name)
					case "value":
						return readString(r, &a.Value)
					}
					return false, nil
				})
				if err != nil {
					return err
				}
				f.Args = append(f.Args, a)
				return nil
			})
		}
		return false, nil
	})
	if err != nil {
		return f, err
	}
	return f, Accepts(f.Wire)
}

// OpenedFrame is host→guest: the answer to exchange. A token names the
// exchange the host opened; Err names a refusal, and a refused exchange has
// no token because there is nothing to poll.
type OpenedFrame struct {
	Wire  string
	Token uint64
	Err   string
}

// EncodeFields writes the frame's keys in declaration order.
func (f OpenedFrame) EncodeFields(w *codec.Writer) {
	w.Key("wire")
	w.String(f.Wire)
	w.Key("token")
	w.Uint64(f.Token)
	w.Key("err")
	w.String(f.Err)
}

// MarshalJSON renders the frame without reflection.
func (f OpenedFrame) MarshalJSON() ([]byte, error) { return marshal(f) }

// DecodeOpened reads an opened frame and gates its version.
func DecodeOpened(data []byte) (OpenedFrame, error) {
	var f OpenedFrame
	err := codec.DecodeObject(data, func(key string, r *codec.Reader) (bool, error) {
		switch key {
		case "wire":
			return readString(r, &f.Wire)
		case "token":
			v, err := r.Uint64()
			f.Token = v
			return true, err
		case "err":
			return readString(r, &f.Err)
		}
		return false, nil
	})
	if err != nil {
		return f, err
	}
	return f, Accepts(f.Wire)
}

// AckFrame is host→guest: the fast beat. Pending is an honest "not yet",
// never an error — only the party who declared comprehension receives, and
// they may not have received yet.
type AckFrame struct {
	Wire  string
	State string // StatePending or StateReceived
	By    string // the comprehender, once there is one
}

// EncodeFields writes the frame's keys in declaration order.
func (f AckFrame) EncodeFields(w *codec.Writer) {
	w.Key("wire")
	w.String(f.Wire)
	w.Key("state")
	w.String(f.State)
	w.Key("by")
	w.String(f.By)
}

// MarshalJSON renders the frame without reflection.
func (f AckFrame) MarshalJSON() ([]byte, error) { return marshal(f) }

// DecodeAck reads an ack frame and gates its version.
func DecodeAck(data []byte) (AckFrame, error) {
	var f AckFrame
	err := codec.DecodeObject(data, func(key string, r *codec.Reader) (bool, error) {
		switch key {
		case "wire":
			return readString(r, &f.Wire)
		case "state":
			return readString(r, &f.State)
		case "by":
			return readString(r, &f.By)
		}
		return false, nil
	})
	if err != nil {
		return f, err
	}
	return f, Accepts(f.Wire)
}

// ValueFrame is host→guest: is the exchange filled, breached, or still
// pending. Breached carries the typed outcome variant of the expected
// response — the future went the other way — and never a transport error,
// because there is no transport in this vocabulary to report.
type ValueFrame struct {
	Wire  string
	State string // StatePending, StateFilled or StateBreached
	By    string
	Body  []byte // the answer, projected to the caller's lineage
	Err   string // the outcome variant, when breached
}

// EncodeFields writes the frame's keys in declaration order.
func (f ValueFrame) EncodeFields(w *codec.Writer) {
	w.Key("wire")
	w.String(f.Wire)
	w.Key("state")
	w.String(f.State)
	w.Key("by")
	w.String(f.By)
	w.Key("body")
	writeRaw(w, f.Body)
	w.Key("err")
	w.String(f.Err)
}

// MarshalJSON renders the frame without reflection.
func (f ValueFrame) MarshalJSON() ([]byte, error) { return marshal(f) }

// DecodeValue reads a value frame and gates its version.
func DecodeValue(data []byte) (ValueFrame, error) {
	var f ValueFrame
	err := codec.DecodeObject(data, func(key string, r *codec.Reader) (bool, error) {
		switch key {
		case "wire":
			return readString(r, &f.Wire)
		case "state":
			return readString(r, &f.State)
		case "by":
			return readString(r, &f.By)
		case "body":
			return readRaw(r, &f.Body)
		case "err":
			return readString(r, &f.Err)
		}
		return false, nil
	})
	if err != nil {
		return f, err
	}
	return f, Accepts(f.Wire)
}

// frame is what every frame in this package can do.
type frame interface{ EncodeFields(*codec.Writer) }

func marshal(f frame) ([]byte, error) {
	var w codec.Writer
	w.BeginObject()
	f.EncodeFields(&w)
	w.EndObject()
	return w.Bytes(), nil
}

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
