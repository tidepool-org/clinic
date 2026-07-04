package postgres

import (
	"bytes"
	"encoding/json"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/bsonrw"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// NormalizeDocument converts a value to the canonical map representation used
// for jsonb payload columns. The value is round-tripped through BSON so typed
// domain structs (live dual writes) and raw documents read back from Mongo
// (backfill) produce identical JSON. Typed values are encoded honoring JSON
// struct tags, matching the store client's BSON options — a plain
// bson.Marshal would produce different keys than what Mongo stores for
// fields that only carry json tags.
func NormalizeDocument(v interface{}) (bson.M, error) {
	buf := new(bytes.Buffer)
	vw, err := bsonrw.NewBSONValueWriter(buf)
	if err != nil {
		return nil, err
	}
	encoder, err := bson.NewEncoder(vw)
	if err != nil {
		return nil, err
	}
	encoder.UseJSONStructTags()
	if err := encoder.Encode(v); err != nil {
		return nil, err
	}
	var m bson.M
	if err := bson.Unmarshal(buf.Bytes(), &m); err != nil {
		return nil, err
	}
	return NormalizeValue(m).(bson.M), nil
}

// NormalizeValue recursively replaces BSON-specific types with values that
// have a stable JSON representation: primitive.DateTime becomes time.Time in
// UTC (RFC 3339 in JSON) and arrays/maps are normalized element-wise.
// primitive.ObjectID is kept as is; it marshals to its hex form.
func NormalizeValue(v interface{}) interface{} {
	switch value := v.(type) {
	case primitive.DateTime:
		return value.Time().UTC()
	case time.Time:
		return value.UTC()
	case bson.M:
		for k, item := range value {
			value[k] = NormalizeValue(item)
		}
		return value
	case map[string]interface{}:
		for k, item := range value {
			value[k] = NormalizeValue(item)
		}
		return value
	case bson.A:
		for i, item := range value {
			value[i] = NormalizeValue(item)
		}
		return value
	case []interface{}:
		for i, item := range value {
			value[i] = NormalizeValue(item)
		}
		return value
	default:
		return v
	}
}

// MarshalPayload normalizes a value and marshals it to the JSON stored in
// jsonb payload columns.
func MarshalPayload(v interface{}) ([]byte, error) {
	normalized, err := NormalizeDocument(v)
	if err != nil {
		return nil, err
	}
	return json.Marshal(normalized)
}
