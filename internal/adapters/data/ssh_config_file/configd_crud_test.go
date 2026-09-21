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

package ssh_config_file

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Adembc/lazyssh/internal/core/domain"
	"go.uber.org/zap"
)

// setupTestRepo creates a temp ssh dir with a main config (Include only)
// and two config.d files, then returns the Repository.
func setupTestRepo(t *testing.T) (*Repository, string) {
	t.Helper()
	home := t.TempDir()
	sshDir := filepath.Join(home, ".ssh")
	if err := os.MkdirAll(filepath.Join(sshDir, "config.d"), 0o755); err != nil {
		t.Fatal(err)
	}
	mainCfg := filepath.Join(sshDir, "config")
	if err := os.WriteFile(mainCfg, []byte("Include ~/.ssh/config.d/*.conf\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(sshDir, "config.d", "dc_payx.conf"), []byte("Host DC_PayX_web_101\n    HostName 172.30.1.101\n    User root\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(sshDir, "config.d", "default.conf"), []byte("Host home_01\n    HostName 192.168.1.10\n    User mac\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	logger := zap.NewNop().Sugar()
	repo := NewRepositoryWithFS(logger, mainCfg, filepath.Join(home, ".lazyssh", "metadata.json"), DefaultFileSystem{}).(*Repository)
	return repo, sshDir
}

func TestAddServerToExistingConfigD(t *testing.T) {
	repo, sshDir := setupTestRepo(t)

	srv := domain.Server{Alias: "DC_PayX_new_1", Host: "172.30.1.200", User: "root", Port: 22}
	if err := repo.AddServer(srv, "dc_payx.conf"); err != nil {
		t.Fatalf("AddServer to dc_payx.conf failed: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(sshDir, "config.d", "dc_payx.conf"))
	if err != nil {
		t.Fatal(err)
	}
	if !contains(string(data), "DC_PayX_new_1") {
		t.Errorf("dc_payx.conf should contain the new host, got:\n%s", data)
	}
}

func TestAddServerToDefaultConfigD(t *testing.T) {
	repo, sshDir := setupTestRepo(t)

	// NEW host with empty source -> should go to config.d/default.conf, NOT main config
	srv := domain.Server{Alias: "brand_new_box", Host: "10.0.0.99", User: "mac", Port: 22}
	if err := repo.AddServer(srv, ""); err != nil {
		t.Fatalf("AddServer with empty source failed: %v", err)
	}

	// Main config must NOT contain the host
	mainData, _ := os.ReadFile(filepath.Join(sshDir, "config"))
	if contains(string(mainData), "brand_new_box") {
		t.Errorf("main config should NOT contain new host, got:\n%s", mainData)
	}

	// default.conf must contain it
	defData, err := os.ReadFile(filepath.Join(sshDir, "config.d", "default.conf"))
	if err != nil {
		t.Fatal(err)
	}
	if !contains(string(defData), "brand_new_box") {
		t.Errorf("default.conf should contain new host, got:\n%s", defData)
	}
}

func TestAddServerCreatesNewConfigD(t *testing.T) {
	repo, sshDir := setupTestRepo(t)

	srv := domain.Server{Alias: "extra_host", Host: "10.1.1.1", User: "ops", Port: 22}
	if err := repo.AddServer(srv, "team_x.conf"); err != nil {
		t.Fatalf("AddServer to new team_x.conf failed: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(sshDir, "config.d", "team_x.conf"))
	if err != nil {
		t.Fatalf("team_x.conf should have been created: %v", err)
	}
	if !contains(string(data), "extra_host") {
		t.Errorf("team_x.conf should contain new host, got:\n%s", data)
	}
}

func TestDeleteServerFromConfigD(t *testing.T) {
	repo, sshDir := setupTestRepo(t)

	// Verify host exists first
	servers, err := repo.ListServers("")
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, s := range servers {
		if s.Alias == "DC_PayX_web_101" {
			found = true
			if s.SourceFile != "dc_payx.conf" {
				t.Errorf("expected SourceFile=dc_payx.conf, got %q", s.SourceFile)
			}
		}
	}
	if !found {
		t.Fatal("DC_PayX_web_101 should exist before delete")
	}

	if err := repo.DeleteServer(domain.Server{Alias: "DC_PayX_web_101", SourceFile: "dc_payx.conf"}); err != nil {
		t.Fatalf("DeleteServer from dc_payx.conf failed: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(sshDir, "config.d", "dc_payx.conf"))
	if err != nil {
		t.Fatal(err)
	}
	if contains(string(data), "DC_PayX_web_101") {
		t.Errorf("dc_payx.conf should NOT contain deleted host, got:\n%s", data)
	}
}

func contains(haystack, needle string) bool {
	return strings.Contains(haystack, needle)
}
