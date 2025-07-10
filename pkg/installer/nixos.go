package installer

import (
	"fmt"
	"log"
	"path"

	config "github.com/MAHDTech/nixos-installer/pkg/config"
	sysutil "github.com/MAHDTech/nixos-installer/pkg/sysutil"
)

// generateNixOSConfig runs nixos-generate-config.
func generateNixOSConfig(execute bool, mountPoint string) error {

	log.Println("Generating NixOS configuration...")

	_, err := sysutil.Execute(
		execute,
		sysutil.ModeNormal,
		"nixos-generate-config",
		"--force",
		"--root",
		mountPoint,
	)
	if err != nil {
		return fmt.Errorf("failed to generate NixOS configuration: %w", err)
	}

	log.Println("NixOS configuration generated.")
	return nil
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

		_, err := sysutil.Execute(
			execute,
			sysutil.ModeNormal,
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
