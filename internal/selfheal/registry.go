package selfheal

import "fmt"

type Registry struct {
	policies map[ErrorCode]Policy
}

func NewRegistry(policies ...Policy) (*Registry, error) {
	r := &Registry{policies: make(map[ErrorCode]Policy, len(policies))}
	for _, policy := range policies {
		if policy == nil {
			return nil, fmt.Errorf("self-heal policy is nil")
		}
		definition := policy.Definition()
		if definition.Code == "" || policy.Name() == "" {
			return nil, fmt.Errorf("self-heal policy name and code are required")
		}
		if _, exists := r.policies[definition.Code]; exists {
			return nil, fmt.Errorf("duplicate self-heal policy for %s", definition.Code)
		}
		r.policies[definition.Code] = policy
	}
	return r, nil
}

func (r *Registry) Lookup(code ErrorCode) (Policy, bool) {
	if r == nil {
		return nil, false
	}
	policy, ok := r.policies[code]
	return policy, ok
}
