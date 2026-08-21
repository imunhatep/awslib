package cloudcontrol

import "encoding/gob"

// init registers the containers that a Cloud Control property bag decodes into.
//
// Unlike the other services in this library, whose entities embed typed SDK
// structs, the entities here hold their properties as map[string]interface{} —
// gob encodes concrete types by reflection but refuses values in an
// interface-typed field without an explicit registration, so caching a resource
// whose properties contain a nested object or array fails outright with
// "gob: type not registered for interface: map[string]interface {}".
//
// Registering both containers covers arbitrarily nested JSON: every value the
// standard decoder can produce is either one of these two, or a scalar
// (string/float64/bool) or nil, all of which gob already handles.
func init() {
	gob.Register(map[string]interface{}{})
	gob.Register([]interface{}{})
}
