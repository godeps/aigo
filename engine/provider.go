package engine

// ProviderConfig describes a named engine preset from a provider.
type ProviderConfig struct {
	Name        string      // registration name, e.g. "alibabacloud-image"
	Engine      Engine      // ready-to-use engine instance
	EnvVars     []string    // required env vars; if empty, always register
	DisplayName DisplayName // localized display name
}

// Provider groups multiple engine presets from a single vendor.
type Provider struct {
	Name    string           // vendor name, e.g. "alibabacloud"
	Configs []ProviderConfig // engine presets offered by this vendor
}
