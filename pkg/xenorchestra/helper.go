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
	"fmt"
	"strings"
	"unicode"

	"github.com/gofrs/uuid"

	"github.com/vatesfr/xenorchestra-go-sdk/pkg/payloads"
)

// convertLittleEndianUUID converts a little-endian UUID to big-endian.
//
// Due to a bug in XCP-ng UEFI supports, SMBIOS 2.8 is used instead of SMBIOS 2.4
// UUIDs are sent in big-endian format instead of little-endian (Microsoft GUID format) as
// mandated by SMBIOS 2.8. When Linux reads the SMBIOS table, it interprets the UUID
// as little-endian, causing a mismatch between the UUID reported by the guest OS
// (via dmidecode/SystemUUID) and the actual VM UUID in Xen Orchestra.
//
// This function swaps the first 8 bytes to convert from the
// incorrectly-interpreted little-endian format to the correct big-endian format.
//
// Reference: https://xcp-ng.org/forum/topic/11078/vm-uuid-via-dmidecode-does-not-match-vm-id-in-xen-orchestra/23
func convertLittleEndianUUID(u uuid.UUID) uuid.UUID {
	var result uuid.UUID
	copy(result[:], u[:])

	// Swap bytes for first field (4 bytes)
	result[0], result[1], result[2], result[3] = u[3], u[2], u[1], u[0]

	// Swap bytes for second field (2 bytes)
	result[4], result[5] = u[5], u[4]

	// Swap bytes for third field (2 bytes)
	result[6], result[7] = u[7], u[6]

	// Last 8 bytes remain unchanged
	return result
}

// getInstanceType returns the instance type for the given VM.
// It returns the instance type name if it matches the expected format, otherwise it returns a formatted string
// returns: the instance type name or a formatted string with vCPU and memory, and an error if any
func getInstanceType(vm *payloads.VM) string {
	memory := float64(vm.Memory.Size) / float64(1024*1024*1024)

	return fmt.Sprintf("%.0fvCPU-%.0fGB",
		float64(vm.CPUs.Max),
		memory)
}

// sanitizeToLabel replaces characters in a string so that it matches the regex:
// (([A-Za-z0-9][-A-Za-z0-9_.]*)?[A-Za-z0-9])?
// and truncates to 63 chars.
func sanitizeToLabel(s string) string {
	// Replace all invalid chars with '-'
	var b strings.Builder

	for _, r := range s {
		if unicode.IsLetter(r) || unicode.IsDigit(r) || r == '-' || r == '_' || r == '.' {
			b.WriteRune(r)
		} else {
			b.WriteRune('-')
		}
	}

	out := b.String()

	// Remove leading and trailing non-alphanumeric
	out = strings.TrimFunc(out, func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsDigit(r)
	})

	const maxLen = 63
	if len(out) > maxLen {
		out = out[:maxLen]
	}

	return out
}
