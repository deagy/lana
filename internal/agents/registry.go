// Package agents provides a local-only registry and durable work queue for
// explicitly named Lana agent roles. It never launches a shell or resolves a
// remote provider; callers supply an Executor when work should be performed.
package agents

import (
	"errors"
	"fmt"
	"sort"
	"strings"
)

// Role describes one supported local agent role. IDs are stable API values.
type Role struct {
	ID           string   `json:"id"`
	Name         string   `json:"name"`
	Description  string   `json:"description"`
	Capabilities []string `json:"capabilities,omitempty"`
}

// Registry is an immutable set of roles accepted by a Queue.
type Registry struct{ roles map[string]Role }

// NewRegistry validates and copies roles. Duplicate IDs are rejected.
func NewRegistry(roles ...Role) (Registry, error) {
	registry := Registry{roles: make(map[string]Role, len(roles))}
	for _, role := range roles {
		if err := validateRole(role); err != nil {
			return Registry{}, err
		}
		if _, exists := registry.roles[role.ID]; exists {
			return Registry{}, fmt.Errorf("duplicate agent role %q", role.ID)
		}
		role.Capabilities = append([]string(nil), role.Capabilities...)
		registry.roles[role.ID] = role
	}
	return registry, nil
}

// MustRegistry is intended for static package role declarations.
func MustRegistry(roles ...Role) Registry {
	registry, err := NewRegistry(roles...)
	if err != nil {
		panic(err)
	}
	return registry
}

// DefaultRegistry contains only local workflow roles. Their task data is
// recorded as JSON and is not treated as executable text.
func DefaultRegistry() Registry {
	return defaultRegistry
}

var defaultRegistry = MustRegistry(
	Role{ID: "planner", Name: "Planner", Description: "Turns structured objectives and context into a local work plan.", Capabilities: []string{"planning", "task-breakdown"}},
	Role{ID: "implementer", Name: "Implementer", Description: "Performs a bounded implementation task supplied as structured input.", Capabilities: []string{"implementation", "validation"}},
	Role{ID: "reviewer", Name: "Reviewer", Description: "Reviews a completed local task and records findings.", Capabilities: []string{"review", "findings"}},
)

// List returns roles sorted by stable ID.
func (r Registry) List() []Role {
	roles := make([]Role, 0, len(r.roles))
	for _, role := range r.roles {
		role.Capabilities = append([]string(nil), role.Capabilities...)
		roles = append(roles, role)
	}
	sort.Slice(roles, func(i, j int) bool { return roles[i].ID < roles[j].ID })
	return roles
}

// Describe returns a role by ID.
func (r Registry) Describe(id string) (Role, bool) {
	role, ok := r.roles[id]
	role.Capabilities = append([]string(nil), role.Capabilities...)
	return role, ok
}

func (r Registry) Has(id string) bool { _, ok := r.roles[id]; return ok }

func validateRole(role Role) error {
	if !validID(role.ID) {
		return fmt.Errorf("invalid agent role ID %q", role.ID)
	}
	if strings.TrimSpace(role.Name) == "" {
		return fmt.Errorf("agent role %q name is required", role.ID)
	}
	if strings.TrimSpace(role.Description) == "" {
		return fmt.Errorf("agent role %q description is required", role.ID)
	}
	return nil
}

func validID(value string) bool {
	if value == "" || len(value) > 96 {
		return false
	}
	for index, r := range value {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-' {
			if r == '-' && (index == 0 || index == len(value)-1) {
				return false
			}
			continue
		}
		return false
	}
	return true
}

var ErrRoleNotFound = errors.New("agent role not found")
