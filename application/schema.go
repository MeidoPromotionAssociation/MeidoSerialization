package application

import (
	"fmt"
	"strings"

	editingv1 "github.com/MeidoPromotionAssociation/MeidoSerialization/schemas/editing/v1"
)

// SchemaDocument is the versioned editing-JSON contract for one registered
// format. JSON is returned as bytes so callers can hand it directly to a
// standard JSON Schema validator or code generator without a lossy map round
// trip.
type SchemaDocument struct {
	FormatID       string
	Representation Representation
	Version        string
	ID             string
	Dialect        string
	MediaType      string
	SHA256         string
	NativeSuffixes []string
	JSON           []byte
}

func (r *Registry) FormatSchema(formatID string) (SchemaDocument, error) {
	formatID = strings.ToLower(strings.TrimSpace(formatID))
	if formatID == "" {
		return SchemaDocument{}, opError("get format schema", CodeInvalidArgument, fmt.Errorf("format ID is required"))
	}
	format, ok := r.Lookup(formatID)
	if !ok {
		return SchemaDocument{}, opError("get format schema", CodeNotFound, fmt.Errorf("format %q is not registered", formatID))
	}
	if !format.Capability.Convert {
		return SchemaDocument{}, opError("get format schema", CodeUnsupported, fmt.Errorf("format %q has no editing JSON representation", format.ID))
	}
	document, found, err := editingv1.Lookup(format.ID)
	if err != nil {
		return SchemaDocument{}, opError("get format schema", CodeInternal, err)
	}
	if !found {
		return SchemaDocument{}, opError("get format schema", CodeUnsupported, fmt.Errorf("format %q has no published editing schema", format.ID))
	}
	return SchemaDocument{
		FormatID:       document.FormatID,
		Representation: RepresentationEditingJSON,
		Version:        document.Version,
		ID:             document.ID,
		Dialect:        document.Dialect,
		MediaType:      document.MediaType,
		SHA256:         document.SHA256,
		NativeSuffixes: append([]string(nil), format.NativeSuffixes...),
		JSON:           append([]byte(nil), document.JSON...),
	}, nil
}

func (r *Registry) GetFormatSchema(formatID string) (SchemaDocument, error) {
	return r.FormatSchema(formatID)
}

func (e *Engine) FormatSchema(formatID string) (SchemaDocument, error) {
	if e == nil || e.registry == nil {
		return SchemaDocument{}, opError("get format schema", CodeInternal, fmt.Errorf("engine is not initialized"))
	}
	return e.registry.FormatSchema(formatID)
}

func (e *Engine) GetFormatSchema(formatID string) (SchemaDocument, error) {
	return e.FormatSchema(formatID)
}
