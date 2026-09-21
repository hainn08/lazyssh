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
	"fmt"
	"os"
	"path/filepath"

	"github.com/Adembc/lazyssh/internal/core/domain"
	"github.com/Adembc/lazyssh/internal/core/ports"
	"github.com/kevinburke/ssh_config"
	"go.uber.org/zap"
)

// Repository implements ServerRepository interface for SSH config file operations.
type Repository struct {
	configPath      string
	configDir       string // ~/.ssh/config.d directory
	fileSystem      FileSystem
	metadataManager *metadataManager
	logger          *zap.SugaredLogger
}

// NewRepository creates a new SSH config repository.
func NewRepository(logger *zap.SugaredLogger, configPath, metaDataPath string) ports.ServerRepository {
	configDir := filepath.Join(filepath.Dir(configPath), "config.d")
	return &Repository{
		logger:          logger,
		configPath:      configPath,
		configDir:       configDir,
		fileSystem:      DefaultFileSystem{},
		metadataManager: newMetadataManager(metaDataPath, logger),
	}
}

// NewRepositoryWithFS creates a new SSH config repository with a custom filesystem.
func NewRepositoryWithFS(logger *zap.SugaredLogger, configPath string, metaDataPath string, fs FileSystem) ports.ServerRepository {
	configDir := filepath.Join(filepath.Dir(configPath), "config.d")
	return &Repository{
		logger:          logger,
		configPath:      configPath,
		configDir:       configDir,
		fileSystem:      fs,
		metadataManager: newMetadataManager(metaDataPath, logger),
	}
}

// ListServers returns all servers matching the query pattern.
// Empty query returns all servers.
// Loads from both ~/.ssh/config and ~/.ssh/config.d/*.conf, merging with
// main config taking precedence for duplicate aliases.
func (r *Repository) ListServers(query string) ([]domain.Server, error) {
	bundle, err := r.loadAllConfigs()
	if err != nil {
		return nil, fmt.Errorf("failed to load config: %w", err)
	}

	// Map main config hosts first (they take precedence)
	servers := r.toDomainServer(bundle.main, domain.SourceFileMain)

	// Map config.d hosts — main config wins for duplicates
	for filename, cfg := range bundle.includes {
		sourceFile := filename // bare filename like "dc_payx.conf"
		extra := r.toDomainServer(cfg, sourceFile)
		servers = r.mergeServersWithDedup(servers, extra, filename)
	}

	metadata, err := r.metadataManager.loadAll()
	if err != nil {
		r.logger.Warnf("Failed to load metadata: %v", err)
		metadata = make(map[string]ServerMetadata)
	}
	servers = r.mergeMetadata(servers, metadata)
	if query == "" {
		return servers, nil
	}

	return r.filterServers(servers, query), nil
}

// mergeServersWithDedup merges extra servers into existing, with existing taking precedence.
// When a duplicate alias is found, the existing server wins and a warning is logged.
func (r *Repository) mergeServersWithDedup(existing, extra []domain.Server, sourceFile string) []domain.Server {
	existingMap := make(map[string]int, len(existing))
	for i, s := range existing {
		existingMap[s.Alias] = i
	}

	for _, s := range extra {
		if _, exists := existingMap[s.Alias]; exists {
			// Duplicate alias — existing (main config) wins, log the conflict
			r.logger.Warnf("duplicate alias '%s' in %s, overridden by %s", s.Alias, sourceFile, domain.SourceFileMain)
			continue
		}
		existing = append(existing, s)
	}

	return existing
}

// AddServer adds a new server to the SSH config.
// The sourceFile parameter specifies which config file to write to:
// - "" or "default" → ~/.ssh/config.d/default.conf (created if doesn't exist)
// - "dc_payx.conf" → ~/.ssh/config.d/dc_payx.conf (created if doesn't exist)
// - "~/.ssh/config" → main config
func (r *Repository) AddServer(server domain.Server, sourceFile string) error {
	// Determine which config file to write to.
	// Empty / "default" group always maps to config.d/default.conf (main config is
	// only used when explicitly requested via SourceFileMain).
	targetFile := sourceFile
	writeToConfigD := true
	if targetFile == "" || targetFile == "default" {
		targetFile = "default.conf"
	} else if targetFile == domain.SourceFileMain {
		writeToConfigD = false
	}

	var cfg *ssh_config.Config
	var err error

	if writeToConfigD {
		// Load or create the config.d file
		cfg, err = r.loadConfigDFile(targetFile)
		if err != nil {
			// If file doesn't exist, create a new config
			if os.IsNotExist(err) {
				cfg = &ssh_config.Config{}
			} else {
				return fmt.Errorf("failed to load config.d file %s: %w", targetFile, err)
			}
		}
	} else {
		cfg, err = r.loadConfig()
		if err != nil {
			return fmt.Errorf("failed to load config: %w", err)
		}
	}

	if r.serverExists(cfg, server.Alias) {
		return fmt.Errorf("server with alias '%s' already exists", server.Alias)
	}

	host := r.createHostFromServer(server)
	cfg.Hosts = append(cfg.Hosts, host)

	// Save to the appropriate file
	if writeToConfigD {
		if err := r.saveConfigDFile(targetFile, cfg); err != nil {
			r.logger.Warnf("Failed to save config.d file %s while adding server: %v", targetFile, err)
			return fmt.Errorf("failed to save config.d file: %w", err)
		}
	} else {
		if err := r.saveConfig(cfg); err != nil {
			r.logger.Warnf("Failed to save config while adding server: %v", err)
			return fmt.Errorf("failed to save config: %w", err)
		}
	}

	return r.metadataManager.updateServer(server, server.Alias)
}

// UpdateServer updates an existing server in the SSH config.
// If the server is from a config.d file, updates that specific file instead of main config.
func (r *Repository) UpdateServer(server domain.Server, newServer domain.Server) error {
	// Determine which config file to update based on server's SourceFile
	sourceFile := server.SourceFile
	isConfigD := sourceFile != "" && sourceFile != domain.SourceFileMain

	var cfg *ssh_config.Config
	var err error

	if isConfigD {
		// Load the specific config.d file
		cfg, err = r.loadConfigDFile(sourceFile)
		if err != nil {
			return fmt.Errorf("failed to load config.d file %s: %w", sourceFile, err)
		}
	} else {
		// Load main config
		cfg, err = r.loadConfig()
		if err != nil {
			return fmt.Errorf("failed to load config: %w", err)
		}
	}

	host := r.findHostByAlias(cfg, server.Alias)
	if host == nil {
		return fmt.Errorf("server with alias '%s' not found", server.Alias)
	}

	if server.Alias != newServer.Alias {
		if r.serverExists(cfg, newServer.Alias) {
			return fmt.Errorf("server with alias '%s' already exists", newServer.Alias)
		}

		newPatterns := make([]*ssh_config.Pattern, 0, len(host.Patterns))
		for _, pattern := range host.Patterns {
			if pattern.Str == server.Alias {
				newPatterns = append(newPatterns, &ssh_config.Pattern{Str: newServer.Alias})
			} else {
				newPatterns = append(newPatterns, pattern)
			}
		}

		host.Patterns = newPatterns

	}

	r.updateHostNodes(host, newServer)

	// Save to the appropriate file
	if isConfigD {
		if err := r.saveConfigDFile(sourceFile, cfg); err != nil {
			r.logger.Warnf("Failed to save config.d file %s while updating server: %v", sourceFile, err)
			return fmt.Errorf("failed to save config.d file: %w", err)
		}
	} else {
		if err := r.saveConfig(cfg); err != nil {
			r.logger.Warnf("Failed to save config while updating server: %v", err)
			return fmt.Errorf("failed to save config: %w", err)
		}
	}

	// Update metadata; pass old alias to allow inline migration
	return r.metadataManager.updateServer(newServer, server.Alias)
}

// DeleteServer removes a server from the SSH config.
// If the server is from a config.d file, removes it from that specific file.
func (r *Repository) DeleteServer(server domain.Server) error {
	// Determine which config file to delete from
	sourceFile := server.SourceFile
	isConfigD := sourceFile != "" && sourceFile != domain.SourceFileMain

	var cfg *ssh_config.Config
	var err error

	if isConfigD {
		cfg, err = r.loadConfigDFile(sourceFile)
		if err != nil {
			return fmt.Errorf("failed to load config.d file %s: %w", sourceFile, err)
		}
	} else {
		cfg, err = r.loadConfig()
		if err != nil {
			return fmt.Errorf("failed to load config: %w", err)
		}
	}

	initialCount := len(cfg.Hosts)
	cfg.Hosts = r.removeHostByAlias(cfg.Hosts, server.Alias)

	if len(cfg.Hosts) == initialCount {
		return fmt.Errorf("server with alias '%s' not found", server.Alias)
	}

	// Save to the appropriate file
	if isConfigD {
		if err := r.saveConfigDFile(sourceFile, cfg); err != nil {
			r.logger.Warnf("Failed to save config.d file %s while deleting server: %v", sourceFile, err)
			return fmt.Errorf("failed to save config.d file: %w", err)
		}
	} else {
		if err := r.saveConfig(cfg); err != nil {
			r.logger.Warnf("Failed to save config while deleting server: %v", err)
			return fmt.Errorf("failed to save config: %w", err)
		}
	}

	return r.metadataManager.deleteServer(server.Alias)
}

// SetPinned sets or unsets the pinned status of a server.
func (r *Repository) SetPinned(alias string, pinned bool) error {
	return r.metadataManager.setPinned(alias, pinned)
}

// RecordSSH increments the SSH access count and updates the last seen timestamp for a server.
func (r *Repository) RecordSSH(alias string) error {
	return r.metadataManager.recordSSH(alias)
}
