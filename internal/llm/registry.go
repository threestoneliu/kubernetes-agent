package llm

// ProviderStatus is the public view of a single provider's
// availability for the /healthz endpoint. The HTTP layer renders it
// directly into JSON, so the field names are the wire shape.
type ProviderStatus struct {
	Name   string `json:"name"`
	Status string `json:"status"`
}

// Registry is the bundle of configured providers plus their current
// reachability. The HTTP layer reads Status() to build the /healthz
// response and to surface a "provider down" indicator in the UI.
//
// Providers carry their own Client (built once at startup). Disabled
// providers have a nil Client and Status == "disabled".
type Registry struct {
	Providers []Provider
	Clients   map[string]Client
	// Health is the latest PingAll snapshot. Map key is provider
	// name. Providers not in the map are unknown (never pinged).
	Health map[string]PingStatus
}

// Status returns one row per configured provider, in the order they
// were declared. The status string is "enabled" for healthy
// providers, "disabled" for unreachable / unconfigured ones, and
// "unknown" for entries that have not been pinged yet.
func (r *Registry) Status() []ProviderStatus {
	out := make([]ProviderStatus, 0, len(r.Providers))
	for _, p := range r.Providers {
		out = append(out, ProviderStatus{
			Name:   p.Name,
			Status: providerStatusString(p, r.Health),
		})
	}
	return out
}

func providerStatusString(p Provider, health map[string]PingStatus) string {
	st, ok := health[p.Name]
	if !ok {
		return "unknown"
	}
	if !st.OK {
		return "disabled"
	}
	return "enabled"
}
