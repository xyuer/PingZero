package config

type GlobalConfig struct {
	ForcedHosts []string           `yaml:"forced_hosts" json:"forcedHosts"`
	Bypass      []BypassRuleConfig `yaml:"bypass" json:"bypass"`
}
