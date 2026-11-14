package telemetry_test

import (
	"testing"

	"task-manager/pkg/telemetry"
)

func TestNormalizePath(t *testing.T) {
	testCases := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "root path",
			input:    "/",
			expected: "/",
		},
		{
			name:     "static route without params",
			input:    "/api/v1/tasks",
			expected: "/api/v1/tasks",
		},
		{
			name:     "route with numeric ID",
			input:    "/api/v1/tasks/42",
			expected: "/api/v1/tasks/:id",
		},
		{
			name:     "route with nested numeric IDs",
			input:    "/api/v1/users/7/tasks/105",
			expected: "/api/v1/users/:id/tasks/:id",
		},
		{
			name:     "route with alphanumeric segment untouched",
			input:    "/api/v1/tasks/active",
			expected: "/api/v1/tasks/active",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			actual := telemetry.NormalizePath(tc.input)
			if actual != tc.expected {
				t.Errorf("NormalizePath(%q) = %q; expected %q", tc.input, actual, tc.expected)
			}
		})
	}
}
