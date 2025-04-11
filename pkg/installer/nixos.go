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

	_, err := utils.Execute(
		execute,
		utils.ModeNormal,
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
		nixOSHostIDString, stdOutErr = utils.Execute(
			execute, // Should always be true if we need the output
			utils.ModeStdOut,
			"head",
			"-c",
			"8",
			"/etc/machine-id",
		)
		if stdOutErr != nil {
			return fmt.Errorf("failed to determine NixOS hostId via head command: %w", stdOutErr)
		}
		log.Printf("Generated hostId: %s\n", nixOSHostIDString)
	}

	// Replace placeholder or default hostId in the configuration content
	configContent := string(nixOSConfigDefault)
	configContent = replaceOrSetHostID(
		configContent,
		nixOSHostIDString,
	)

	// Write the modified configuration file.
	err = os.WriteFile(nixOSConfigPath, []byte(configContent), 0600) // Restrictive permissions
	if err != nil {
		return fmt.Errorf("failed to write modified configuration.nix: %w", err)
	}

	log.Println("NixOS configuration modified.")
	return nil
}

// TODO: Implement this function.
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
	execute,
	executeInstall bool,
	mountPoint string,
	configData *config.Config,
) error {
	mountPointNixOSConfig := path.Join(
		mountPoint,
		"/etc/nixos",
	)

	if executeInstall {

		log.Println("--- Starting NixOS installation ---")

		log.Println(
			"Ensure NIXPKGS_ALLOW_UNFREE=1 is exported in your environment if your flake requires unfree packages.",
		)

		_, err := utils.Execute(
			execute,
			utils.ModeNormal,
			"nixos-install",
			"--verbose",
			"--no-root-passwd",
			"--root",
			mountPoint,
			"--flake",
			configData.NixOS.Flake,
		)
		if err != nil {
			return fmt.Errorf("failed during nixos-install execution: %w", err)
		}

		log.Println("--- Finished NixOS installation ---")

	} else {

		// Print instructions for manual installation
		fmt.Println("")
		fmt.Println("---------------------------------------------------------------------")
		fmt.Println("NixOS installation skipped, manual installation instructions below.")
		fmt.Println("---------------------------------------------------------------------")
		fmt.Println("")
		fmt.Println("The NixOS system configuration has been prepared. Review the generated configuration if desired:")
		fmt.Println("")
		fmt.Printf("\tHardware Config: %s/etc/nixos/hardware-configuration.nix\n", mountPoint)
		fmt.Printf("\tMain Config:     %s/etc/nixos/configuration.nix\n", mountPoint)
		fmt.Println("")
		fmt.Println("To install NixOS manually, run the following commands:")
		fmt.Println("")
		fmt.Println("\texport NIXPKGS_ALLOW_UNFREE=1  # If using unfree packages")
		fmt.Printf("\tsudo -E nixos-install --verbose --no-root-passwd --root %s --flake %s\n", mountPoint, configData.NixOS.Flake)
		fmt.Println("")
		fmt.Println("Notes:")
		fmt.Println("\t- You can edit the configuration files before running nixos-install.")
		fmt.Println("\t- Re-run nixos-install if you make further changes before rebooting.")
		if configData.NixOS.Config.Enabled {
			fmt.Printf("\t- TIP: If using the NixOS config partition, consider copying your flake source folder into %s\n", mountPointNixOSConfig)
		}
		fmt.Println("\t- REMINDER: Double-check disk IDs in hardware-configuration.nix before the final install!")
		fmt.Println("")
		fmt.Println("---------------------------------------------------------------------")
		fmt.Println("")
	}

	return nil
}
