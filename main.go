// Main entry point for the NixOS Installer.
package main

import (
	"log"

	installer "github.com/MAHDTech/nixos-installer/pkg/installer"
)

// Run the NixOS Installer.
func main() {

	log.Println("Starting NixOS Installer...")

	err := installer.Run()
	if err != nil {
		log.Fatalf("Installation failed with error: %v", err)
	}

	log.Println("Installation completed successfully!")
}
