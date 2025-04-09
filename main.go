// Main entry point for the NixOS Installer.
package main

import (
	"log"

	"github.com/MAHDTech/nixos-installer/pkg/installer"
)

// Run the NixOS Installer.
func main() {
	err := installer.Run()
	if err != nil {
		log.Fatalf("Installation failed: %v", err)
	}
}
