package envoy

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

type hcmHttpFilterInput struct {
	authn            bool
	authzClusterName string
}

func TestHcmHttpFilters(t *testing.T) {
	var tests = []struct {
		name   string
		inputs *hcmHttpFilterInput
		length int
	}{
		{
			name: "test has router filter only",
			inputs: &hcmHttpFilterInput{
				authn:            false,
				authzClusterName: "",
			},
			length: 1,
		},
		{
			name: "test has authn filter",
			inputs: &hcmHttpFilterInput{
				authn:            true,
				authzClusterName: "",
			},
			length: 2,
		},
		{
			name: "test has authz filter",
			inputs: &hcmHttpFilterInput{
				authn:            false,
				authzClusterName: "local_opa",
			},
			length: 2,
		},
		{
			name: "test has both authn/authz filter",
			inputs: &hcmHttpFilterInput{
				authn:            true,
				authzClusterName: "local_opa",
			},
			length: 3,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			filters := hcmHttpFilters(tc.inputs.authn, tc.inputs.authzClusterName)
			assert.Len(t, filters, tc.length)
		})
	}
}
