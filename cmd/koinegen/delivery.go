package main

import (
	"fmt"
	"go/format"
	"sort"
	"strconv"
	"strings"
)

// A delivery is the projected facts a station is constructed with, plus the
// verbs its stratum can reach. Its fields are FLAT — the whole lineage
// written out at this stratum — because the wire is flat: the host projects
// to a lineage and hands the guest one object. Flat fields are also what
// lets an author write the delivery as a literal in a test, which is the
// harness's whole promise.

func (g *generator) deliveryFile(ns *Namespace) ([]byte, error) {
	var b strings.Builder
	b.WriteString(header(ns))
	fmt.Fprintf(&b, "package %s\n\n", ns.Package)

	imports := map[string]bool{koinePath: true, codecPath: true, selectorPath: true}
	needStrconv := false
	for _, d := range ns.Deliveries {
		for _, of := range ownedFields(d.Projects()) {
			if name, ok := strings.CutPrefix(of.Type, "ref:"); ok {
				if refOwner := of.owner.Owner().RefOwner(name); refOwner != ns {
					imports[g.importPath(refOwner)] = true
				}
			}
		}
		for _, v := range d.Verbs {
			for _, in := range v.Intents {
				for _, p := range in.Params {
					if p.Type == "int" || p.Type == "bool" {
						needStrconv = true
					}
				}
			}
		}
	}
	if needStrconv {
		imports["strconv"] = true
	}
	writeImports(&b, imports)

	handles := map[string]*Type{}
	for _, d := range ns.Deliveries {
		g.writeAwaitFunc(&b, d)
		g.writeDelivery(&b, ns, d)
		for _, v := range d.Verbs {
			g.writeVerb(&b, ns, d, v)
			for _, in := range v.Intents {
				handles[in.ReturnType().Name] = in.ReturnType()
			}
		}
	}
	names := make([]string, 0, len(handles))
	for name := range handles {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		g.writeHandle(&b, handles[name])
	}
	return format.Source([]byte(b.String()))
}

// writeAwaitFunc emits the selector constructor a station names in Awaits().
// The delivery and the selector are generated from the same registry entry,
// so the shape a station declares and the type it is handed cannot drift.
func (g *generator) writeAwaitFunc(b *strings.Builder, d *Delivery) {
	fn := d.AwaitFunc()
	comment(b, fn+" is the shape a station awaits to be handed a "+d.Name+".")
	fmt.Fprintf(b, "func %s() selector.Selector {\n\treturn ", fn)
	switch d.Await.Mode {
	case "event":
		fmt.Fprintf(b, "selector.Event(%q)", d.Await.Type)
	case "resolved":
		fmt.Fprintf(b, "selector.Resolved(%q)", d.Await.Type)
	case "absent":
		fmt.Fprintf(b, "selector.Absent(selector.Event(%q))", d.Await.Type)
	}
	if d.Await.Anchor != "" {
		fmt.Fprintf(b, ".At(%q)", d.Await.Anchor)
	}
	b.WriteString("\n}\n\n")
}

func (g *generator) writeDelivery(b *strings.Builder, ns *Namespace, d *Delivery) {
	fields := ownedFields(d.Projects())
	comment(b, d.Name+" "+d.Doc, "Every field of it is yours and nothing beyond it exists here: the border of the kingdom is this struct, and what a stratum does not contain does not exist for it.")
	fmt.Fprintf(b, "type %s struct {\n\tkoine.IsDelivery\n\n", d.Name)
	for _, of := range fields {
		fmt.Fprintf(b, "\t%s %s\n", of.Name, g.goType(of.owner.Owner(), of.Type, ns))
	}
	b.WriteString("\n\t// broker is the host standing below this station: unexported and\n")
	b.WriteString("\t// unreachable, because a body speaks verbs, never a broker.\n\tbroker koine.Broker\n}\n\n")

	comment(b, "Projects names the utterance type this delivery is cut from.")
	fmt.Fprintf(b, "func (%s) Projects() string { return %q }\n\n", d.Name, d.Projects().Event)
	comment(b, "Namespace is the stratum this delivery projects to.")
	fmt.Fprintf(b, "func (%s) Namespace() string { return %q }\n\n", d.Name, ns.Namespace)

	comment(b, "Bind wires the delivery to the host standing below it. Construction is delivery: the host binds at construction and koine/testing binds from the script. A station body never calls this — there is nothing in a body that holds a Broker to pass.")
	fmt.Fprintf(b, "func (d %s) Bind(b koine.Broker) koine.Delivery {\n\td.broker = b\n\treturn d\n}\n\n", d.Name)

	comment(b, "EncodeFields writes the delivery's whole kingdom, lineage first.")
	fmt.Fprintf(b, "func (d %s) EncodeFields(w *codec.Writer) {\n", d.Name)
	for _, of := range fields {
		writeEncodeField(b, "d."+of.Name, of.Field)
	}
	b.WriteString("}\n\n")

	comment(b, "DecodeField reads one projected key; anything else is skipped.")
	fmt.Fprintf(b, "func (d *%s) DecodeField(key string, r *codec.Reader) (bool, error) {\n", d.Name)
	if len(fields) > 0 {
		b.WriteString("\tswitch key {\n")
		for _, of := range fields {
			fmt.Fprintf(b, "\tcase %q:\n", of.JSON)
			writeDecodeField(b, "\t\t", "d."+of.Name, of.Field, g.goType(of.owner.Owner(), of.Type, ns))
		}
		b.WriteString("\t}\n")
	}
	b.WriteString("\treturn false, nil\n}\n\n")

	comment(b, "MarshalJSON renders the delivery without reflection.")
	fmt.Fprintf(b, "func (d %s) MarshalJSON() ([]byte, error) {\n", d.Name)
	b.WriteString("\tvar w codec.Writer\n\tw.BeginObject()\n\td.EncodeFields(&w)\n\tw.EndObject()\n\treturn w.Bytes(), nil\n}\n\n")

	comment(b, "UnmarshalJSON reads the delivery from JSON already cut to its lineage.")
	fmt.Fprintf(b, "func (d *%s) UnmarshalJSON(data []byte) error {\n", d.Name)
	b.WriteString("\treturn codec.DecodeObject(data, d.DecodeField)\n}\n\n")

	for _, v := range d.Verbs {
		comment(b, v.Name+" "+v.Doc, "It opens the "+strconv.Quote(v.Seat)+" seat. An unfilled seat is refused at registration by name — the honest terminus, before runtime — never discovered in here.")
		fmt.Fprintf(b, "func (d %s) %s() %s {\n\treturn %s{broker: d.broker}\n}\n\n",
			d.Name, v.Name, verbTypeName(d, v), verbTypeName(d, v))
	}
}

func verbTypeName(d *Delivery, v *Verb) string { return v.Name + "Seat" }

func (g *generator) writeVerb(b *strings.Builder, ns *Namespace, d *Delivery, v *Verb) {
	name := verbTypeName(d, v)
	comment(b, name+" is the "+strconv.Quote(v.Seat)+" seat, curried with the station's context at construction. A body speaks intents at it; it never names a tool and it never names an address. The operator owns the address and the credential, and the deployment binds the client.")
	fmt.Fprintf(b, "type %s struct{ broker koine.Broker }\n\n", name)

	for _, in := range v.Intents {
		params := make([]string, 0, len(in.Params))
		for _, p := range in.Params {
			params = append(params, paramName(p.Name)+" "+g.goType(ns, p.Type, ns))
		}
		comment(b, in.Name+" "+in.Doc, "It speaks "+strconv.Quote(in.Exchange)+" — an intent, uttered at the call. How you use the handle IS the branch control: Value() consumed is inline, spoken and never consumed is concurrent, koine.Detach is detached.")
		fmt.Fprintf(b, "func (v %s) %s(%s) koine.Handle[%s] {\n", name, in.Name, strings.Join(params, ", "), in.ReturnType().Name)
		fmt.Fprintf(b, "\th := &%s{\n\t\tbroker: v.broker,\n\t\texchange: koine.Exchange{\n", handleTypeName(in.ReturnType()))
		fmt.Fprintf(b, "\t\t\tSeat: %q,\n\t\t\tName: %q,\n", v.Seat, in.Exchange)
		if len(in.Params) > 0 {
			b.WriteString("\t\t\tArgs: []koine.Arg{\n")
			for _, p := range in.Params {
				fmt.Fprintf(b, "\t\t\t\t{Name: %q, Value: %s},\n", p.Name, argValue(p))
			}
			b.WriteString("\t\t\t},\n")
		}
		b.WriteString("\t\t},\n\t}\n\th.speak()\n\treturn h\n}\n\n")
	}
}

func argValue(p Param) string {
	n := paramName(p.Name)
	switch p.Type {
	case "string":
		return n
	case "int":
		return "strconv.Itoa(" + n + ")"
	case "bool":
		return "strconv.FormatBool(" + n + ")"
	default: // outcome
		return "string(" + n + ")"
	}
}

func handleTypeName(t *Type) string { return "handle" + t.Name }

// writeHandle emits the Handle implementation for one answer type. The
// intent is spoken when the verb is called; the handle is what is left to
// gate on. Value materializes and tells the broker it was consumed — the
// beat that lets a conformance test prove the manifest's consumption reading
// matches what the body actually did.
func (g *generator) writeHandle(b *strings.Builder, t *Type) {
	name := handleTypeName(t)
	comment(b, name+" gates on an exchange answering with "+t.Name+".")
	fmt.Fprintf(b, "type %s struct {\n\tbroker   koine.Broker\n\texchange koine.Exchange\n\tanswer   *koine.Answer\n}\n\n", name)

	comment(b, "speak utters the intent once. With no host below the station it panics: an exchange never fails silently, and a nil answer is not an answer.")
	fmt.Fprintf(b, "func (h *%s) speak() {\n", name)
	b.WriteString("\tif h.answer != nil {\n\t\treturn\n\t}\n")
	b.WriteString("\tif h.broker == nil {\n\t\tpanic(\"koine: \" + h.exchange.Name + \" spoke with no host below this station — an exchange never fails silently\")\n\t}\n")
	b.WriteString("\ta := h.broker.Speak(h.exchange)\n\th.answer = &a\n}\n\n")

	comment(b, "Received is the fast beat: someone who declared comprehension has this now.")
	fmt.Fprintf(b, "func (h *%s) Received() koine.Ack {\n\th.speak()\n\treturn koine.Ack{By: h.answer.By}\n}\n\n", name)

	comment(b, "Value gates on completion and materializes the answer. The error is the typed outcome variant of the expected response, never transport — branch on it, don't defend against it.")
	fmt.Fprintf(b, "func (h *%s) Value() (%s, error) {\n", name, t.Name)
	fmt.Fprintf(b, "\th.speak()\n\th.broker.Consumed(h.exchange)\n\tvar v %s\n", t.Name)
	b.WriteString("\tif h.answer.Err != nil {\n\t\treturn v, h.answer.Err\n\t}\n")
	b.WriteString("\tif len(h.answer.JSON) == 0 {\n\t\treturn v, nil\n\t}\n")
	b.WriteString("\tif err := v.UnmarshalJSON(h.answer.JSON); err != nil {\n\t\treturn v, err\n\t}\n")
	b.WriteString("\treturn v, nil\n}\n\n")
}
