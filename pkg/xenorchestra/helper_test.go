/*
Copyright 2025 Vatesfr

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

	http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/
package xenorchestra

import (
	"testing"

	"github.com/gofrs/uuid"
	"github.com/stretchr/testify/assert"
)

func TestConvertLittleEndianUUID(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "All zeros",
			input:    "00000000-0000-0000-0000-000000000000",
			expected: "00000000-0000-0000-0000-000000000000",
		},
		{
			name:     "All ones",
			input:    "ffffffff-ffff-ffff-ffff-ffffffffffff",
			expected: "ffffffff-ffff-ffff-ffff-ffffffffffff",
		},
		{
			name:     "Example 1",
			input:    "59aee6ae-f0b8-a2bd-a89d-55638d1e9725",
			expected: "aee6ae59-b8f0-bda2-a89d-55638d1e9725",
		},
		{
			name:     "Example 2",
			input:    "6c0bc35f-7e48-34fe-04c3-67812e3b17a7",
			expected: "5fc30b6c-487e-fe34-04c3-67812e3b17a7",
		},
		{
			name:     "Example 3",
			input:    "77f4080b-1a49-82a9-23c4-d224723624ea",
			expected: "0b08f477-491a-a982-23c4-d224723624ea",
		},
		{
			name:     "Example 4",
			input:    "6a87cb0f-ca4c-ffa5-3ca2-fc398fb25eac",
			expected: "0fcb876a-4cca-a5ff-3ca2-fc398fb25eac",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			input, err := uuid.FromString(tt.input)
			assert.NoError(t, err, "Failed to parse input UUID")

			expected, err := uuid.FromString(tt.expected)
			assert.NoError(t, err, "Failed to parse expected UUID")

			result := convertLittleEndianUUID(input)

			assert.Equal(t, expected, result, "Converted UUID does not match expected value")
		})
	}
}
