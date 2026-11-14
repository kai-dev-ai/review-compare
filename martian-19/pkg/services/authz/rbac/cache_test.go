package rbac

import (
	"testing"

	"github.com/grafana/authlib/types"
)

func TestUserPermDenialCacheKey(t *testing.T) {
	// Each tuple must retain its own entry, including formerly ambiguous boundaries.
	tuples := [][6]string{
		{"org-12", string(types.TypeUser), "uid", "dashboards:read", "a_b", "c"},
		{"org-12", string(types.TypeUser), "uid", "dashboards:read", "a", "b_c"},
		{"org-12", string(types.TypeServiceAccount), "uid", "dashboards:read", "a_b", "c"},
		{"org-13", string(types.TypeUser), "uid", "dashboards:read", "a_b", "c"},
		{"org-12", string(types.TypeUser), "other", "dashboards:read", "a_b", "c"},
		{"org-12", string(types.TypeUser), "uid", "dashboards:write", "a_b", "c"},
		{"org-12", string(types.TypeUser), "uid", "dashboards:read", "", "_"},
		{"org-12", string(types.TypeUser), "uid", "dashboards:read", "_", ""},
		{"org-12", string(types.TypeUser), "uid", "dashboards:read", "a:1", "b"},
		{"org-12", string(types.TypeUser), "uid", "dashboards:read", "a", "1:b"},
	}
	seen := make(map[string][6]string)
	for _, tuple := range tuples {
		key := userPermDenialCacheKey(tuple[0], types.IdentityType(tuple[1]), tuple[2], tuple[3], tuple[4], tuple[5])
		if previous, ok := seen[key]; ok {
			t.Errorf("cache key collision between %q and %q", previous, tuple)
		}
		seen[key] = tuple
		if repeated := userPermDenialCacheKey(tuple[0], types.IdentityType(tuple[1]), tuple[2], tuple[3], tuple[4], tuple[5]); repeated != key {
			t.Errorf("unstable key for %q", tuple)
		}
	}
}
