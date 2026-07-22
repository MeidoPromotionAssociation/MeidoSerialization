package application

import (
	"fmt"
	"strings"

	knowledgev1 "github.com/MeidoPromotionAssociation/MeidoSerialization/schemas/knowledge/v1"
)

type GuideDocument struct {
	FormatID  string
	Version   string
	ID        string
	MediaType string
	SHA256    string
	SchemaID  string
	Coverage  string
	JSON      []byte
}

func (r *Registry) FormatGuide(formatID string) (GuideDocument, error) {
	formatID = strings.ToLower(strings.TrimSpace(formatID))
	if formatID == "" {
		return GuideDocument{}, opError("get format guide", CodeInvalidArgument, fmt.Errorf("format ID is required"))
	}
	format, ok := r.Lookup(formatID)
	if !ok {
		return GuideDocument{}, opError("get format guide", CodeNotFound, fmt.Errorf("format %q is not registered", formatID))
	}
	if !format.Capability.Convert {
		return GuideDocument{}, opError("get format guide", CodeUnsupported, fmt.Errorf("format %q has no editing JSON representation", format.ID))
	}
	schema, err := r.FormatSchema(format.ID)
	if err != nil {
		return GuideDocument{}, err
	}
	document, err := knowledgev1.Resolve(format.ID, schema.ID, schema.JSON)
	if err != nil {
		return GuideDocument{}, opError("get format guide", CodeInternal, err)
	}
	return GuideDocument{
		FormatID: document.FormatID, Version: document.Version, ID: document.ID,
		MediaType: document.MediaType, SHA256: document.SHA256, SchemaID: document.SchemaID,
		Coverage: document.Coverage, JSON: append([]byte(nil), document.JSON...),
	}, nil
}

func (r *Registry) GetFormatGuide(formatID string) (GuideDocument, error) {
	return r.FormatGuide(formatID)
}

func (e *Engine) FormatGuide(formatID string) (GuideDocument, error) {
	if e == nil || e.registry == nil {
		return GuideDocument{}, opError("get format guide", CodeInternal, fmt.Errorf("engine is not initialized"))
	}
	return e.registry.FormatGuide(formatID)
}

func (e *Engine) GetFormatGuide(formatID string) (GuideDocument, error) {
	return e.FormatGuide(formatID)
}
