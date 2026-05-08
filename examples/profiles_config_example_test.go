package examples

// ProfileSlicesConfig is the config shape for the layered string-slice example.
//
// Each field defaults to []string{"abc"}. The loaders will then leave
// DefaultOnlyProfiles alone, replace FileProfiles from TOML, replace EnvProfiles
// from an environment variable, and replace FlagProfiles from pflags.
type ProfileSlicesConfig struct {
	DefaultOnlyProfiles []string `toml:"default_only_profiles"`
	FileProfiles        []string `toml:"file_profiles"`
	EnvProfiles         []string `toml:"env_profiles" config:"env-profiles" help:"profiles loaded from environment"`
	FlagProfiles        []string `toml:"flag_profiles" config:"flag-profiles" help:"profiles loaded from pflags"`
}

// ProfileSlicesDefaults contains the defaults for the layered string-slice example.
var ProfileSlicesDefaults = ProfileSlicesConfig{
	DefaultOnlyProfiles: []string{"abc"},
	FileProfiles:        []string{"abc"},
	EnvProfiles:         []string{"abc"},
	FlagProfiles:        []string{"abc"},
}
