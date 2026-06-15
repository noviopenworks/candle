package main

import (
	"fmt"

	"github.com/vend-ai/intel-mcp/internal/version"
)

func main() {
	fmt.Println("intel-mcp", version.String())
}
