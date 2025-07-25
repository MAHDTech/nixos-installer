package installer

import (
	"encoding/json"
	"fmt"
	"path"

	config "github.com/MAHDTech/nixos-installer/pkg/config"
	sysutil "github.com/MAHDTech/nixos-installer/pkg/sysutil"
)

// checkMountpoints captures and returns the current system mountpoints.
// Returns an error if lsblk fails.
func checkMountpoints(execute bool) ([]byte, error) {

	var mountpointsString string
	var mountpointsJSON []byte
	var err error

	sysutil.Info("Checking existing mountpoints...")

	// Get the mountpoints as a string
	mountpointsString, err = sysutil.Execute(
		execute,
		sysutil.ModeStdOut,
		"lsblk",
		"--noheadings",
		"--json",
		"--output",
		"ID,MOUNTPOINTS",
	)
	if err != nil {
		return nil, fmt.Errorf("failed to execute lsblk to check mountpoints: %w", err)
	}

	// Unmarshal the string into a JSON object
	mountpointsJSON, err = json.Marshal(mountpointsString)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal mountpoints string: %w", err)
	}

	// Return the JSON object
	return mountpointsJSON, nil
}

// createDirectories creates the necessary temporary mount directories.
// Returns an error if any directory creation fails.
func createDirectories(execute bool, mountPoint string, configData *config.Config) error {

	sysutil.Info("Creating base mount directory structure...")

	// Create the main mount directory.
	sysutil.Info("Creating mount directory %s", mountPoint)
	_, err := sysutil.Execute(
		execute,
		sysutil.ModeNormal,
		"mkdir",
		"-p",
		mountPoint,
	)
	if err != nil {
		return fmt.Errorf("failed to create base mount directory %s: %w", mountPoint, err)
	}

	// Create the 'boot' mount point.
	mountPointBoot := path.Join(mountPoint, "boot")
	sysutil.Info("Creating mount point for 'boot' at: %s", mountPointBoot)
	_, err = sysutil.Execute(
		execute,
		sysutil.ModeNormal,
		"mkdir",
		"-p",
		mountPointBoot,
	)
	if err != nil {
		return fmt.Errorf("failed to create boot directory %s: %w", mountPointBoot, err)
	}

	// Create the 'efi' mount point.
	mountPointUEFI := path.Join(mountPoint, "boot/efi")
	sysutil.Info("Creating mount point for 'efi' at: %s", mountPointUEFI)
	_, err = sysutil.Execute(
		execute,
		sysutil.ModeNormal,
		"mkdir",
		"-p",
		mountPointUEFI,
	)
	if err != nil {
		return fmt.Errorf("failed to create UEFI directory %s: %w", mountPointUEFI, err)
	}

	// Create the 'nixos' configuration mount point if enabled.
	if configData.NixOS.Config.Enabled {
		mountPointNixOSConfig := path.Join(mountPoint, "boot/nixos")
		sysutil.Info("Creating mount point for 'nixos' at: %s", mountPointNixOSConfig)
		_, err = sysutil.Execute(
			execute,
			sysutil.ModeNormal,
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

	// Create the 'home' mount point.
	mountPointHome := path.Join(mountPoint, "home")
	sysutil.Info("Creating mount point for 'home' at: %s", mountPointHome)
	_, err = sysutil.Execute(
		execute,
		sysutil.ModeNormal,
		"mkdir",
		"-p",
		mountPointHome,
	)
	if err != nil {
		return fmt.Errorf("failed to create home directory %s: %w", mountPointHome, err)
	}

	// Create the 'nix' mount point.
	mountPointNix := path.Join(mountPoint, "nix")
	sysutil.Info("Creating mount point for 'nix' at: %s", mountPointNix)
	_, err = sysutil.Execute(
		execute,
		sysutil.ModeNormal,
		"mkdir",
		"-p",
		mountPointNix,
	)
	if err != nil {
		return fmt.Errorf("failed to create nix directory %s: %w", mountPointNix, err)
	}

	// Create the 'var' mount point.
	mountPointVar := path.Join(mountPoint, "var")
	sysutil.Info("Creating mount point for 'var' at: %s", mountPointVar)
	_, err = sysutil.Execute(
		execute,
		sysutil.ModeNormal,
		"mkdir",
		"-p",
		mountPointVar,
	)
	if err != nil {
		return fmt.Errorf("failed to create var directory %s: %w", mountPointVar, err)
	}

	// Create the 'lib' mount point.
	mountPointLib := path.Join(mountPoint, "var/lib")
	sysutil.Info("Creating mount point for 'lib' at: %s", mountPointLib)
	_, err = sysutil.Execute(
		execute,
		sysutil.ModeNormal,
		"mkdir",
		"-p",
		mountPointLib,
	)
	if err != nil {
		return fmt.Errorf("failed to create lib directory %s: %w", mountPointLib, err)
	}

	// Create the 'tmp' mount point.
	mountPointTmp := path.Join(mountPoint, "tmp")
	sysutil.Info("Creating mount point for 'tmp' at: %s", mountPointTmp)
	_, err = sysutil.Execute(
		execute,
		sysutil.ModeNormal,
		"mkdir",
		"-p",
		mountPointTmp,
	)
	if err != nil {
		return fmt.Errorf("failed to create tmp directory %s: %w", mountPointTmp, err)
	}

	// Log the completion of the directory structure creation.
	sysutil.Info("Base directory structure created.")
	return nil
}

// createOptionalDirectories creates the optional mount directories.
// Returns an error if any directory creation fails.
func createOptionalDirectories(execute bool, mountPoint string, configData *config.Config) error {

	var err error

	sysutil.Info("Creating optional mount directory structure...")

	// Create the 'docker' mount point if enabled.
	if configData.Containers.Enabled {
		mountPointDocker := path.Join(mountPoint, "var/lib/docker")
		sysutil.Info("Creating mount point for 'docker' at: %s", mountPointDocker)
		_, err = sysutil.Execute(
			execute,
			sysutil.ModeNormal,
			"mkdir",
			"-p",
			mountPointDocker,
		)
		if err != nil {
			return fmt.Errorf("failed to create docker directory %s: %w", mountPointDocker, err)
		}
	}

	// Create the 'containers' mount point if enabled.
	if configData.Containers.Enabled {
		mountPointContainers := path.Join(mountPoint, "var/lib/containers")
		sysutil.Info("Creating mount point for 'containers' at: %s", mountPointContainers)
		_, err = sysutil.Execute(
			execute,
			sysutil.ModeNormal,
			"mkdir",
			"-p",
			mountPointContainers,
		)
		if err != nil {
			return fmt.Errorf(
				"failed to create containers directory %s: %w",
				mountPointContainers,
				err,
			)
		}
	}

	// Create the 'incus' mount point if enabled.
	if configData.Incus.Enabled {
		mountPointIncus := path.Join(mountPoint, "var/lib/incus")
		sysutil.Info("Creating mount point for 'incus' at: %s", mountPointIncus)
		_, err = sysutil.Execute(
			execute,
			sysutil.ModeNormal,
			"mkdir",
			"-p",
			mountPointIncus,
		)
		if err != nil {
			return fmt.Errorf("failed to create incus directory %s: %w", mountPointIncus, err)
		}
	}

	// Create the 'incus/storage-pools' mount point if enabled.
	if configData.Incus.Enabled {
		mountPointIncusStoragePools := path.Join(mountPoint, "var/lib/incus/storage-pools")
		sysutil.Info(
			"Creating mount point for 'incus/storage-pools' at: %s",
			mountPointIncusStoragePools,
		)
		_, err = sysutil.Execute(
			execute,
			sysutil.ModeNormal,
			"mkdir",
			"-p",
			mountPointIncusStoragePools,
		)
		if err != nil {
			return fmt.Errorf(
				"failed to create incus/storage-pools directory %s: %w",
				mountPointIncusStoragePools,
				err,
			)
		}
	}

	// Log the completion of the directory structure creation.
	sysutil.Info("Base directory structure created.")
	return nil
}
