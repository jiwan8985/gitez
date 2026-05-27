package main

import "gez/cmd"

// version is set at build time via -ldflags "-X main.version=..."
var version = "dev"

func main() {
	cmd.SetVersion(version)
	cmd.Execute()
}
