package config

type GameConfig struct {
	ID              string             `yaml:"id" json:"id"`
	Name            string             `yaml:"name" json:"name"`
	Processes       []string           `yaml:"processes" json:"processes"`
	AccelerateHosts []string           `yaml:"accelerate_hosts" json:"accelerateHosts"`
	Bypass          []BypassRuleConfig `yaml:"bypass" json:"bypass"`
}

type BypassRuleConfig struct {
	Protocol string   `yaml:"protocol" json:"protocol"`
	CIDRs    []string `yaml:"cidrs" json:"cidrs"`
	Ports    []uint16 `yaml:"ports" json:"ports"`
	Comment  string   `yaml:"comment" json:"comment"`
}
