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

	"github.com/Adembc/lazyssh/internal/core/domain"
)

func TestResolveSourceFile(t *testing.T) {
	tests := []struct {
		name      string
		groupName string
		want      string
	}{
		{name: "empty goes to default.conf", groupName: "", want: "default.conf"},
		{name: "default goes to default.conf", groupName: "default", want: "default.conf"},
		{name: "bare group name gets .conf suffix", groupName: "test", want: "test.conf"},
		{name: "group name already has .conf", groupName: "dc_payx.conf", want: "dc_payx.conf"},
		{name: "main config preserved", groupName: domain.SourceFileMain, want: domain.SourceFileMain},
		{name: "group with space", groupName: "my group", want: "my group.conf"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := resolveSourceFile(tt.groupName); got != tt.want {
				t.Errorf("resolveSourceFile(%q) = %q, want %q", tt.groupName, got, tt.want)
			}
		})
	}
}

func TestSourceFileToGroupName(t *testing.T) {
	tests := []struct {
		name       string
		sourceFile string
		want       string
	}{
		{name: "empty goes to default", sourceFile: "", want: "default"},
		{name: "default.conf goes to default", sourceFile: "default.conf", want: "default"},
		{name: "strip .conf suffix", sourceFile: "dc_payx.conf", want: "dc_payx"},
		{name: "main config preserved", sourceFile: domain.SourceFileMain, want: domain.SourceFileMain},
		{name: "group with space", sourceFile: "my group.conf", want: "my group"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := sourceFileToGroupName(tt.sourceFile); got != tt.want {
				t.Errorf("sourceFileToGroupName(%q) = %q, want %q", tt.sourceFile, got, tt.want)
			}
		})
	}
}

func TestResolveSourceFileRoundTrip(t *testing.T) {
	// Ensure resolveSourceFile(sourceFileToGroupName(sf)) == sf for config.d files
	for _, sf := range []string{"default.conf", "dc_payx.conf", "team_x.conf"} {
		group := sourceFileToGroupName(sf)
		resolved := resolveSourceFile(group)
		if resolved != sf {
			t.Errorf("round-trip failed: %q -> group %q -> %q", sf, group, resolved)
		}
	}
}
