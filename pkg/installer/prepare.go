package installer

import (
	"fmt"
	"log"
	"path"

	config "github.com/MAHDTech/nixos-installer/pkg/config"
	utils "github.com/MAHDTech/nixos-installer/pkg/utils"
)

// CheckMountpoints captures and returns the current system mountpoints.
// Returns an error if lsblk fails.
func checkMountpoints(execute bool) ([]byte, error) {
	log.Println("Checking existing mountpoints...")
	mountpointsString, err := utils.ExecuteStdOut(
		execute, // Using execute flag here might not be ideal, stdout capture usually shouldn't change system state. Consider always true?
		"lsblk",
		"--noheadings",
		"--json",
		"--output",
		"ID,MOUNTPOINTS",
	)
	if err != nil {
		return nil, fmt.Errorf("failed to execute lsblk to check mountpoints: %w", err)
	}
	// Convert the string into JSON
	return []byte(mountpointsString), nil
}

// CreateDirectories creates the necessary temporary mount directories.
// Returns an error if any directory creation fails.
func createDirectories(execute bool, mountPoint string, configData *config.Config) error {
	log.Println("Creating base mount directory structure...")

	// Create the main mount directory.
	log.Printf("Creating mount directory %s\n", mountPoint)
	err := utils.Execute(
		execute,
		"mkdir",
		"-p",
		mountPoint,
	)
	if err != nil {
		return fmt.Errorf("failed to create base mount directory %s: %w", mountPoint, err)
	}

	// Create mount point for 'boot'
	mountPointBoot := path.Join(mountPoint, "boot")
	log.Printf("Creating mount point for 'boot' at: %s\n", mountPointBoot)
	err = utils.Execute(
		execute,
		"mkdir",
		"-p",
		mountPointBoot,
	)
	if err != nil {
		return fmt.Errorf("failed to create boot directory %s: %w", mountPointBoot, err)
	}

	// Create mount point for 'efi'
	mountPointUEFI := path.Join(mountPoint, "boot/efi")
	log.Printf("Creating mount point for 'efi' at: %s\n", mountPointUEFI)
	err = utils.Execute(
		execute,
		"mkdir",
		"-p",
		mountPointUEFI,
	)
	if err != nil {
		return fmt.Errorf("failed to create UEFI directory %s: %w", mountPointUEFI, err)
	}

	// Create mount point for 'nixos' configuration if enabled.
	if configData.NixOS.Config.Enabled {
		mountPointNixOSConfig := path.Join(mountPoint, "boot/nixos")
		log.Printf("Creating mount point for 'nixos-config' at: %s\n", mountPointNixOSConfig)
		err = utils.Execute(
			execute,
			"mkdir",
			"-p",
			mountPointNixOSConfig,
		)
		if err != nil {
			return fmt.Errorf(
				"failed to create NixOS config directory %s: %w",
				mountPointNixOSConfig,
				err,
			)
		}
	}

	// Create mount point for 'home'
	mountPointHome := path.Join(mountPoint, "home")
	log.Printf("Creating mount point for 'home' at: %s\n", mountPointHome)
	err = utils.Execute(
		execute,
		"mkdir",
		"-p",
		mountPointHome,
	)
	if err != nil {
		return fmt.Errorf("failed to create home directory %s: %w", mountPointHome, err)
	}

	// Create mount point for 'nix'
	mountPointNix := path.Join(mountPoint, "nix")
	log.Printf("Creating mount point for 'nix' at: %s\n", mountPointNix)
	err = utils.Execute(
		execute,
		"mkdir",
		"-p",
		mountPointNix,
	)
	if err != nil {
		return fmt.Errorf("failed to create nix directory %s: %w", mountPointNix, err)
	}

	// Create mount point for 'var'
	mountPointVar := path.Join(mountPoint, "var")
	log.Printf("Creating mount point for 'var' at: %s\n", mountPointVar)
	err = utils.Execute(
		execute,
		"mkdir",
		"-p",
		mountPointVar,
	)
	if err != nil {
		return fmt.Errorf("failed to create var directory %s: %w", mountPointVar, err)
	}

	// Create mount point for 'lib'
	mountPointLib := path.Join(mountPoint, "var/lib")
	log.Printf(
		"Creating mount point for 'lib' at: %s\n",
		mountPointLib,
	) // Log message says 'var', should be 'lib'
	err = utils.Execute(
		execute,
		"mkdir",
		"-p",
		mountPointLib,
	)
	if err != nil {
		return fmt.Errorf("failed to create lib directory %s: %w", mountPointLib, err)
	}

	// Create mount point for 'docker'
	mountPointDocker := path.Join(mountPoint, "var/lib/docker")
	log.Printf("Creating mount point for 'docker' at: %s\n", mountPointDocker)
	err = utils.Execute(
		execute,
		"mkdir",
		"-p",
		mountPointDocker,
	)
	if err != nil {
		return fmt.Errorf("failed to create docker directory %s: %w", mountPointDocker, err)
	}

	// Create mount point for 'tmp'
	mountPointTmp := path.Join(mountPoint, "tmp")
	log.Printf("Creating mount point for 'tmp' at: %s\n", mountPointTmp)
	err = utils.Execute(
		execute,
		"mkdir",
		"-p",
		mountPointTmp,
	)
	if err != nil {
		return fmt.Errorf("failed to create tmp directory %s: %w", mountPointTmp, err)
	}
	log.Println("Base directory structure created.")
	return nil
}
