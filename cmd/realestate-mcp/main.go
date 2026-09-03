package main

import (
	"fmt"
	"os"
)

func main() {
	if len(os.Args) > 1 && os.Args[1] == "--version" {
		fmt.Println("tw-prop-mcp v2.0.0")
		return
	}
	fmt.Println("tw-prop-mcp: MCP server starting (bootstrap)")
}
