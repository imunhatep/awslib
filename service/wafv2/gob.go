package wafv2

import (
	"encoding/json"

	"github.com/go-errors/errors"
)

// The cache serializes entities with encoding/gob, and gob refuses the WAFv2 SDK
// types outright: a rule statement reaches empty marker structs such as
// types.AllQueryArguments and types.NoneAction, and gob fails with "type has no
// exported fields" for any type graph containing one — whether or not the value
// is set. WebACL, RuleGroup and the logging configuration all reach them.
//
// So the entities that hold rules implement gob.GobEncoder/GobDecoder through
// JSON, which round-trips the SDK structs faithfully (an empty action marshals
// to {} and comes back as a non-nil pointer, which is what distinguishes Allow
// from Block). A wire struct with named fields is used rather than the entity
// itself: the entity embeds both service.AbstractResource and the SDK struct, and
// both declare ARN, which encoding/json resolves by dropping the field.
//
// Pinned by TestEntitiesRoundTripThroughGob.

func gobEncodeJSON(wire any) ([]byte, error) {
	out, err := json.Marshal(wire)
	if err != nil {
		return nil, errors.New(err)
	}

	return out, nil
}

func gobDecodeJSON(data []byte, wire any) error {
	if err := json.Unmarshal(data, wire); err != nil {
		return errors.New(err)
	}

	return nil
}
