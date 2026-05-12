package configreporter

import (
	"bytes"
	"fmt"
	"io"
	"sort"

	"github.com/BurntSushi/toml"
	configloader "github.com/jmcampanini/go-config-loader"
)

// Reporter formats an already-loaded config value and its provenance metadata.
type Reporter[C any] struct {
	config  C
	updates configloader.Updates
}

// New returns a Reporter for config and load report metadata.
//
// New snapshots report updates. The config value is stored by value; map and
// slice fields follow normal Go value semantics.
func New[C any](config C, report configloader.LoadReport) Reporter[C] {
	updatesCopy := make(configloader.Updates, len(report.Updates))
	for path, source := range report.Updates {
		updatesCopy[path] = source
	}
	return Reporter[C]{config: config, updates: updatesCopy}
}

// WriteTOML writes the effective config as TOML.
func (r Reporter[C]) WriteTOML(w io.Writer) error {
	if w == nil {
		return fmt.Errorf("configreporter: writer is nil")
	}
	if err := configloader.ValidateConfig[C](); err != nil {
		return err
	}
	return toml.NewEncoder(w).Encode(r.config)
}

// TOML returns the effective config as TOML bytes.
func (r Reporter[C]) TOML() ([]byte, error) {
	var buf bytes.Buffer
	if err := r.WriteTOML(&buf); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// ProvenanceHeaders returns display headers for provenance rows.
func (r Reporter[C]) ProvenanceHeaders() []string {
	return []string{"Path", "Source"}
}

// ProvenanceRows returns sorted provenance rows as path/source pairs.
func (r Reporter[C]) ProvenanceRows() [][]string {
	paths := make([]string, 0, len(r.updates))
	for path := range r.updates {
		paths = append(paths, path)
	}
	sort.Strings(paths)

	rows := make([][]string, 0, len(paths))
	for _, path := range paths {
		rows = append(rows, []string{path, r.updates[path]})
	}
	return rows
}
