package postgres

import (
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// NormalizeDocument converts a value to the canonical map representation used
// for jsonb payload columns. The value is round-tripped through BSON so typed
// domain structs (live dual writes) and raw documents read back from Mongo
// (backfill) produce identical JSON.
func NormalizeDocument(v interface{}) (bson.M, error) {
	raw, err := bson.Marshal(v)
	if err != nil {
		return nil, err
	}
	var m bson.M
	if err := bson.Unmarshal(raw, &m); err != nil {
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
