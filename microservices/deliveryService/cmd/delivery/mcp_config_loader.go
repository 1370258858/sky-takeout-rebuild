package main

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"regexp"

	"gopkg.in/yaml.v3"
)

// MCPConfig represents the MCP server configuration
type MCPConfig struct {
	Server MCPServerConfig `yaml:"server"`
	Tools  []MCPToolConfig `yaml:"tools"`
}

// MCPServerConfig represents the server settings
type MCPServerConfig struct {
	Name    string `yaml:"name"`
	Version string `yaml:"version"`
	Address string `yaml:"address"`
	Path    string `yaml:"path"`
}

// MCPToolConfig represents a single tool configuration
type MCPToolConfig struct {
	Name        string `yaml:"name"`
	Description string `yaml:"description"`
	Handler     string `yaml:"handler"`
}

// LoadMCPConfig loads MCP configuration from a YAML file
func LoadMCPConfig(configPath string) (*MCPConfig, error) {
	// If configPath is empty, use default location
	if configPath == "" {
		configPath = "mcp-config.yaml"
		// Try to find it in the same directory as the executable
		exe, err := os.Executable()
		if err == nil {
			exePath := filepath.Dir(exe)
			potentialPath := filepath.Join(exePath, "mcp-config.yaml")
			if _, err := os.Stat(potentialPath); err == nil {
				configPath = potentialPath
			}
		}
	}

	log.Printf("loading MCP config from: %s", configPath)

	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read MCP config file: %w", err)
	}

	config := &MCPConfig{}
	if err := yaml.Unmarshal(data, config); err != nil {
		return nil, fmt.Errorf("failed to parse MCP config: %w", err)
	}

	// Expand environment variables in config
	if err := expandEnvVars(config); err != nil {
		return nil, fmt.Errorf("failed to expand environment variables: %w", err)
	}

	return config, nil
}

// expandEnvVars expands environment variables in config strings
func expandEnvVars(config *MCPConfig) error {
	config.Server.Address = expandString(config.Server.Address)
	config.Server.Path = expandString(config.Server.Path)
	return nil
}

// expandString expands environment variables in a string
func expandString(s string) string {
	// Pattern: ${VAR_NAME:default_value} or ${VAR_NAME}
	pattern := regexp.MustCompile(`\$\{([A-Za-z_][A-Za-z0-9_]*):([^}]*)\}|\$\{([A-Za-z_][A-Za-z0-9_]*)\}`)

	return pattern.ReplaceAllStringFunc(s, func(match string) string {
		submatches := pattern.FindStringSubmatch(match)
		if len(submatches) > 0 {
			// ${VAR:default} format
			if submatches[1] != "" {
				varName := submatches[1]
				defaultValue := submatches[2]
				if val, ok := os.LookupEnv(varName); ok {
					return val
				}
				return defaultValue
			}
			// ${VAR} format
			if submatches[3] != "" {
				varName := submatches[3]
				if val, ok := os.LookupEnv(varName); ok {
					return val
				}
				return ""
			}
		}
		return match
	})
}

// PrintConfig logs the loaded configuration
func (c *MCPConfig) PrintConfig() {
	log.Printf("=== MCP Server Config ===")
	log.Printf("Name: %s", c.Server.Name)
	log.Printf("Version: %s", c.Server.Version)
	log.Printf("Address: %s", c.Server.Address)
	log.Printf("Path: %s", c.Server.Path)
	log.Printf("Total Tools: %d", len(c.Tools))
	for _, tool := range c.Tools {
		log.Printf("  - %s (handler: %s)", tool.Name, tool.Handler)
	}
	log.Printf("========================")
}

// ValidateConfig validates the configuration
func (c *MCPConfig) ValidateConfig() error {
	if c.Server.Name == "" {
		return fmt.Errorf("server name is required")
	}
	if c.Server.Version == "" {
		return fmt.Errorf("server version is required")
	}
	if c.Server.Address == "" {
		return fmt.Errorf("server address is required")
	}
	if len(c.Tools) == 0 {
		return fmt.Errorf("at least one tool must be configured")
	}

	// Validate tool names and handlers
	for _, tool := range c.Tools {
		if tool.Name == "" {
			return fmt.Errorf("tool name cannot be empty")
		}
		if tool.Handler == "" {
			return fmt.Errorf("tool %s: handler cannot be empty", tool.Name)
		}
	}

	return nil
}
