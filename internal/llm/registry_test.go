package llm

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRegistry_Status(t *testing.T) {
	reg := &Registry{
		Providers: []Provider{
			{Name: "a"},
			{Name: "b"},
			{Name: "c"},
		},
		Health: map[string]PingStatus{
			"a": {Name: "a", OK: true},
			"b": {Name: "b", OK: false, Reason: "timeout"},
			// c is absent -> unknown
		},
	}
	got := reg.Status()
	assert.Equal(t, []ProviderStatus{
		{Name: "a", Status: "enabled"},
		{Name: "b", Status: "disabled"},
		{Name: "c", Status: "unknown"},
	}, got)
}

func TestRegistry_StatusEmpty(t *testing.T) {
	reg := &Registry{}
	got := reg.Status()
	assert.Empty(t, got)
}
