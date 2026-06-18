package main

import (
	"log"
)

// GetToolByHandler returns the tool configuration by handler name
func GetToolByHandler(config *MCPConfig, handlerName string) *MCPToolConfig {
	for _, tool := range config.Tools {
		if tool.Handler == handlerName {
			return &tool
		}
	}
	return nil
}

// PrintAllTools logs all configured tools
func PrintAllTools(config *MCPConfig) {
	log.Printf("=== Configured MCP Tools ===")
	for _, tool := range config.Tools {
		log.Printf("  - %s (handler: %s)", tool.Name, tool.Handler)
	}
	log.Printf("=============================")
}
