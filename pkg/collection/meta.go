package collection

import "encoding/json"

// Meta describes persisted per-collection metadata.
type Meta struct {
	Name string `json:"name"`
	Type Type   `json:"type,omitempty"`
}

// MarshalList serialises metadata slice.
func MarshalList(metas []Meta) ([]byte, error) {
	return json.MarshalIndent(metas, "", "  ")
}

// UnmarshalList deserialises metadata slice and upgrades legacy arrays of strings.
func UnmarshalList(data []byte) ([]Meta, error) {
	if len(data) == 0 {
		return []Meta{}, nil
	}
	var metas []Meta
	if err := json.Unmarshal(data, &metas); err == nil {
		for i := range metas {
			if metas[i].Type == "" {
				metas[i].Type = TypeGeneric
			}
		}
		return metas, nil
	}
	// Fallback for legacy format (array of strings).
	var legacy []string
	if err := json.Unmarshal(data, &legacy); err != nil {
		return nil, err
	}
	metas = make([]Meta, 0, len(legacy))
	for _, name := range legacy {
		metas = append(metas, Meta{Name: name, Type: TypeGeneric})
	}
	return metas, nil
}
