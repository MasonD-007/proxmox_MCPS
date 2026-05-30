// Package features is the registration hub for all MCP tools, resources, and prompts.
//
// Each feature file in this package registers itself via an init() function,
// so importing this package once in main() is sufficient to wire every tool.
package features
