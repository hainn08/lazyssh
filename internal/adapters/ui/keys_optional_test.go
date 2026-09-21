// Copyright 2025.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package ui

import (
	"testing"
)

// TestBlankKeysPassesValidation ensures the Keys field can be left empty
// (password-only SSH auth), and that non-empty keys are not forced to exist.
func TestBlankKeysPassesValidation(t *testing.T) {
	validators := GetFieldValidators()
	v, ok := validators["Keys"]
	if !ok {
		t.Fatal("Keys validator missing")
	}

	t.Run("empty keys valid", func(t *testing.T) {
		if err := v.Validate(""); err != nil {
			t.Errorf("empty Keys should be valid (password auth), got: %v", err)
		}
	})

	t.Run("whitespace keys valid", func(t *testing.T) {
		if err := v.Validate("   "); err != nil {
			t.Errorf("whitespace Keys should be valid, got: %v", err)
		}
	})

	t.Run("key path without existing file valid", func(t *testing.T) {
		// The key file does not exist on this machine — must NOT block save
		if err := v.Validate("~/.ssh/id_does_not_exist"); err != nil {
			t.Errorf("non-existent key path should be allowed (may live on other host), got: %v", err)
		}
	})

	t.Run("newline rejected", func(t *testing.T) {
		if err := v.Validate("~/.ssh/id_rsa\n"); err == nil {
			t.Errorf("newline in Keys should be rejected")
		}
	})
}

// TestKeysNotRequiredFlag ensures Keys has no Required flag.
func TestKeysNotRequiredFlag(t *testing.T) {
	validators := GetFieldValidators()
	v, ok := validators["Keys"]
	if !ok {
		t.Fatal("Keys validator missing")
	}
	if v.Required {
		t.Errorf("Keys must not be marked Required (password-only hosts allowed)")
	}
}
