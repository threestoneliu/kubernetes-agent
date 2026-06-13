package policy

func DefaultRules() []Rule {
	return []Rule{
		{Name: "deny-delete-system-ns", Effect: Deny, Match: Match{
			Action:    []string{"delete"},
			Namespace: []string{"kube-system", "kube-public", "kube-node-lease"},
		}},
		{Name: "deny-dangerous-kinds", Effect: Deny, Match: Match{
			Action: []string{"apply", "delete"},
			Kind:   []string{"Node", "ClusterRole", "ClusterRoleBinding", "CustomResourceDefinition"},
		}},
		{Name: "deny-privileged", Effect: Deny, Match: Match{
			Action: []string{"apply"},
			UnsafeFields: map[string]any{
				"spec.template.spec.containers[*].securityContext.privileged": true,
				"spec.template.spec.hostNetwork":                              true,
				"spec.template.spec.hostPID":                                  true,
			},
		}},
		{Name: "confirm-production", Effect: Confirm, Match: Match{
			Action:    []string{"apply", "delete", "scale"},
			Namespace: []string{"production", "prod"},
		}},
	}
}
