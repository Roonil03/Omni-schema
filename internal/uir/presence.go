package uir

// Presence distinguishes missing vs explicit-null vs present values.
type Presence int

const (
	PresencePresent Presence = iota
	PresenceNull
	PresenceMissing
	PresenceDefaulted
)

func (p Presence) String() string {
	switch p {
	case PresenceNull:
		return "null"
	case PresenceMissing:
		return "missing"
	case PresenceDefaulted:
		return "defaulted"
	default:
		return "present"
	}
}

// FieldCardinality models requiredness independent of the host protocol.
type FieldCardinality int

const (
	CardinalityOptional FieldCardinality = iota
	CardinalityRequired
)

// BytesPolicy is the declared mapping from UIR_Bytes to text protocols.
type BytesPolicy string

const (
	BytesBase64       BytesPolicy = "base64"
	BytesHex          BytesPolicy = "hex"
	BytesReject       BytesPolicy = "reject"
	BytesCustomScalar BytesPolicy = "scalar"
)

// ConversionKind reports how a field value was converted.
type ConversionKind int

const (
	ConversionLossless ConversionKind = iota
	ConversionSafeCoerce
	ConversionLossy
	ConversionUnsupported
)

func (k ConversionKind) String() string {
	switch k {
	case ConversionSafeCoerce:
		return "safe_coercion"
	case ConversionLossy:
		return "lossy"
	case ConversionUnsupported:
		return "unsupported"
	default:
		return "lossless"
	}
}

// ConversionReport is attached to projection/morph results.
type ConversionReport struct {
	Kind     ConversionKind
	Notes    []string
	Lossy    bool
	Fields   []string
}

func (r *ConversionReport) Add(kind ConversionKind, field, note string) {
	if r == nil {
		return
	}
	if kind > r.Kind {
		r.Kind = kind
	}
	if kind == ConversionLossy || kind == ConversionUnsupported {
		r.Lossy = true
	}
	if field != "" {
		r.Fields = append(r.Fields, field)
	}
	if note != "" {
		r.Notes = append(r.Notes, note)
	}
}
