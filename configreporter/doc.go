// Package configreporter formats already-loaded config values and provenance metadata.
//
// It does not load configuration or mutate values. A Reporter stores the config
// value passed to New and snapshots provenance updates. Map and slice fields in
// the config value follow normal Go value semantics; callers should treat their
// contents as immutable after constructing a Reporter if they need stable output.
package configreporter
