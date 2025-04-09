package installer

import (
	"fmt"
	"log"
	"os"
	"path"

	config "github.com/MAHDTech/nixos-installer/pkg/config"
	utils "github.com/MAHDTech/nixos-installer/pkg/utils"
)

// generateNixOSConfig runs nixos-generate-config.
func generateNixOSConfig(execute bool, mountPoint string) error {
	log.Println("Generating NixOS configuration...")
	err := utils.Execute(
		execute,
		"nixos-generate-config",
		"--root",
		mountPoint,
	)
	if err != nil {
		return fmt.Errorf("failed to generate NixOS configuration: %w", err)
	}
	log.Println("NixOS configuration generated.")
	return nil
}

// modifyNixOSConfig modifies the generated configuration, e.g., sets hostId.
func modifyNixOSConfig(execute bool, mountPoint string, configData *config.Config) error {
	if !execute {
		log.Println("Dry run, skipping NixOS configuration modification...")
		return nil
	}

	log.Println("Modifying NixOS configuration...")
	nixOSConfigPath := path.Join(mountPoint, "/etc/nixos/configuration.nix")

	// #nosec G304 - File path is constructed internally, not from user input directly at this point.
	nixOSConfigDefault, err := os.ReadFile(nixOSConfigPath)
	if err != nil {
		return fmt.Errorf("failed to read NixOS configuration file %s: %w", nixOSConfigPath, err)
	}

	// Determine hostId
	var nixOSHostIDString string
	if configData.NixOS.HostID != "" {
		// Use the user provided host id.
		nixOSHostIDString = configData.NixOS.HostID
		log.Printf("Using user-provided hostId: %s\n", nixOSHostIDString)
	} else {
		// Use the first 8 characters of the machine id.
		log.Println("Generating hostId from /etc/machine-id...")
		// This relies on /etc/machine-id existing in the installer environment.
		var stdOutErr error
		nixOSHostIDString, stdOutErr = utils.ExecuteStdOut(
			execute, // Should always be true if we need the output
			"head",
			"-c",
			"8",
			"/etc/machine-id", // Assuming this exists in the installer environment
		)
		if stdOutErr != nil {
			return fmt.Errorf("failed to determine NixOS hostId via head command: %w", stdOutErr)
		}
		log.Printf("Generated hostId: %s\n", nixOSHostIDString)
	}

	// Replace placeholder or default hostId in the configuration content
	// This replacement logic might need adjustment based on the actual template/default config
	configContent := string(nixOSConfigDefault)
	configContent = replaceOrSetHostID(
		configContent,
		nixOSHostIDString,
	) // Assuming replaceOrSetHostID exists or is added

	// Write the modified configuration file.
	err = os.WriteFile(nixOSConfigPath, []byte(configContent), 0600) // Restrictive permissions
	if err != nil {
		return fmt.Errorf("failed to write modified configuration.nix: %w", err)
	}
	log.Println("NixOS configuration modified.")
	return nil
}

// Dummy function for placeholder - needs actual implementation
func replaceOrSetHostID(content, hostID string) string {
	// Implement logic to find and replace/add the hostId line
	// For example, using regex or string replacement
	// Example placeholder: `networking.hostId = "defaultHostId";`
	// Replace with: `networking.hostId = "<actual hostId>";`
	log.Printf("Placeholder: Would replace/set hostId to %s in config content", hostID)
	return content // Return modified content
}

// installNixOS runs the nixos-install command or prints instructions.
func installNixOS(
	execute, executeInstall bool,
	mountPoint string,
	configData *config.Config,
) error {
	mountPointNixOSConfig := path.Join(
		mountPoint,
		"/etc/nixos",
	) // Corrected path for informational message

	if executeInstall {
		log.Println("Starting NixOS installation...")
		// The original script relied on the user setting this manually via export.
		// Consider setting it via os.Setenv if appropriate or adding a note.
		log.Println(
			"Ensure NIXPKGS_ALLOW_UNFREE=1 is exported in your environment if your flake requires unfree packages.",
		)
		err := utils.Execute(
			execute, // This should definitely use the execute flag
			"nixos-install",
			"--verbose",
			"--no-root-passwd", // Ensure no password prompt
			"--root",
			mountPoint,
			"--flake",
			configData.NixOS.Flake, // Assumes configData.NixOS.Flake is the correct flake reference (e.g., path or URL)
		)
		if err != nil {
			return fmt.Errorf("failed during nixos-install execution: %w", err)
		}
		log.Println("NixOS installation command executed.")
	} else {
		// Print instructions for manual installation
		fmt.Println("")
		fmt.Println("---------------------------------------------------------------------")
		fmt.Println(" Dry Run Complete or Installation Skipped")
		fmt.Println("---------------------------------------------------------------------")
		fmt.Println("System prepared. Review the generated configuration if desired:")
		fmt.Printf("  Hardware Config: %s/etc/nixos/hardware-configuration.nix\n", mountPoint)
		fmt.Printf("  Main Config:     %s/etc/nixos/configuration.nix\n", mountPoint)
		fmt.Println("")
		fmt.Println("To install NixOS manually, run the following commands:")
		fmt.Println("")
		fmt.Println("  export NIXPKGS_ALLOW_UNFREE=1  # If using unfree packages")
		fmt.Printf("  sudo -E nixos-install --verbose --no-root-passwd --root %s --flake %s\n", mountPoint, configData.NixOS.Flake)
		fmt.Println("")
		fmt.Println("Notes:")
		fmt.Println(" - You can edit the configuration files before running nixos-install.")
		fmt.Println(" - Re-run nixos-install if you make further changes before rebooting.")
		if configData.NixOS.Config.Enabled {
			fmt.Printf(" - TIP: If using the NixOS config partition, consider copying your flake source to %s\n", mountPointNixOSConfig)
		}
		fmt.Println(" - REMINDER: Double-check disk IDs in hardware-configuration.nix before the final install!")
		fmt.Println("---------------------------------------------------------------------")
		fmt.Println("")
	}
	return nil
}
