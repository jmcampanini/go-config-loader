// Package pflagloader provides optional pflag integration for configloader.
//
// Fields with config tags are registered as canonical long flags. Slice fields
// may also declare pflag_singular:"name" to register an explicit pflag-only
// alias that appends one non-empty scalar value per occurrence without CSV
// splitting.
package pflagloader

const SourcePFlag = "<pflag>"
