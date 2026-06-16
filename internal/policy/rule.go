package policy

type Effect string

const (
	Allow   Effect = "allow"
	Confirm Effect = "confirm"
	Deny    Effect = "deny"
)

type Rule struct {
	Name   string `yaml:"name"`
	Effect Effect `yaml:"effect"`
	Match  Match  `yaml:"match"`
}

type Match struct {
	Action       []string       `yaml:"action,omitempty"`
	Namespace    []string       `yaml:"namespace,omitempty"`
	Kind         []string       `yaml:"kind,omitempty"`
	UnsafeFields map[string]any `yaml:"unsafeFields,omitempty"`
}
