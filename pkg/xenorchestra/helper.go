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

	"github.com/vatesfr/xenorchestra-go-sdk/pkg/payloads"
)

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
