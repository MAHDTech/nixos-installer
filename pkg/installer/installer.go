package installer

import (
	"flag"
	"fmt"
	"log"
	"os"
	"path"
	"regexp"
	"time"

	config "github.com/MAHDTech/nixos-installer/pkg/config"
	utils "github.com/MAHDTech/nixos-installer/pkg/utils"
	validate "github.com/MAHDTech/nixos-installer/pkg/validate"
)

const mountPoint = "/mnt/nixos"

const zfsPool = "zpool"

const zfsDatasetBoot = "boot"
const zfsDatasetRoot = "root"
const zfsDatasetHome = "home"
const zfsDatasetNixStore = "nix"
const zfsDatasetSwap = "swap"
const zfsDatasetTmp = "tmp"
const zfsDatasetVar = "var"

func Run() {

	// Determine the path to the configuration file.
	configFile := flag.String(
		"config",
		"config.yaml",
		"Path to the YAML configuration file.",
	)

	// By default we run in dry run mode unless the 'dry-run' flag is set to false
	// to avoid a user accidentally making changes.
	execute := flag.Bool(
		"run",
		false,
		"Execute mode. (default is false which only dry runs commands)",
	)

	// By default only the disk partitioning and nix generation is done.
	// To automatically install NixOS, set the 'install' flag to true.
	executeInstall := flag.Bool(
		"install",
		false,
		"Automatically install NixOS. (default is false which only generates the NixOS configuration)",
	)

	// Parse the flags.
	flag.Parse()

	if *execute {
		log.Println("Running in execute mode.")
	} else {
		log.Println("Running in dry run mode, see '-help' for more information.")
	}

	// Read the YAML configuration file and parse it into a Config struct.
	configData, err := config.ReadConfig(*configFile)
	validate.Error(err)

	/*
		##################################################
			Create directories
		##################################################
	*/

	// Create the directories where the temporary mount points will be created.
	log.Printf("Creating mount directory %s\n", mountPoint)
	utils.Execute(
		*execute,
		"mkdir",
		"-p",
		mountPoint,
	)

	// Create mount point for 'boot'
	mountPointBoot := path.Join(mountPoint, "boot")
	log.Printf("Creating mount point for 'boot' at: %s\n", mountPointBoot)
	utils.Execute(
		*execute,
		"mkdir",
		"-p",
		mountPointBoot,
	)

	// Create mount point for 'efi'
	mountPointUEFI := path.Join(mountPoint, "boot/efi")
	log.Printf("Creating mount point for 'efi' at: %s\n", mountPointUEFI)
	utils.Execute(
		*execute,
		"mkdir",
		"-p",
		mountPointUEFI,
	)

	// Create mount point for 'home'
	mountPointHome := path.Join(mountPoint, "home")
	log.Printf("Creating mount point for 'home' at: %s\n", mountPointHome)
	utils.Execute(
		*execute,
		"mkdir",
		"-p",
		mountPointHome,
	)

	// Create mount point for 'nix'
	mountPointNix := path.Join(mountPoint, "nix")
	log.Printf("Creating mount point for 'nix' at: %s\n", mountPointNix)
	utils.Execute(
		*execute,
		"mkdir",
		"-p",
		mountPointNix,
	)

	// Create mount point for 'var'
	mountPointVar := path.Join(mountPoint, "var")
	log.Printf("Creating mount point for 'var' at: %s\n", mountPointVar)
	utils.Execute(
		*execute,
		"mkdir",
		"-p",
		mountPointVar,
	)

	// Create mount point for 'tmp'
	mountPointTmp := path.Join(mountPoint, "tmp")
	log.Printf("Creating mount point for 'tmp' at: %s\n", mountPointTmp)
	utils.Execute(
		*execute,
		"mkdir",
		"-p",
		mountPointTmp,
	)

	/*
		##################################################
			UEFI
		##################################################
	*/

	// Unmount the UEFI target device if it is mounted.
	log.Printf("Unmounting %s if it is mounted.\n", configData.UEFI.Disk)
	utils.ExecuteSilent(
		*execute,
		"umount",
		configData.UEFI.Disk,
	)

	// Zap the UEFI target device.
	log.Printf("Zapping %s.\n", configData.UEFI.Disk)
	utils.Execute(
		*execute,
		"sgdisk",
		"--zap-all",
		configData.UEFI.Disk,
	)

	// Run partprobe to update the partition table.
	log.Println("Running partprobe to update the partition table.")
	utils.ExecuteSilent(
		*execute,
		"partprobe",
	)

	// Prepare the UEFI disk.
	log.Printf("Preparing UEFI disk %s.\n", configData.UEFI.Disk)
	utils.Execute(
		*execute,
		"parted",
		"--script",
		"--fix",
		"--align",
		"optimal",
		configData.UEFI.Disk,
		"--",
		"mklabel",
		"gpt",
	)

	// Create the UEFI partition.
	log.Printf("Creating UEFI partition on %s with label %s and size %s.\n", configData.UEFI.Disk, configData.UEFI.Label, configData.UEFI.Size)
	utils.Execute(
		*execute,
		"parted",
		"--script",
		"--fix",
		"--align",
		"optimal",
		configData.UEFI.Disk,
		"--",
		"mkpart",
		configData.UEFI.Label,
		"fat32",
		"1MiB",
		configData.UEFI.Size,
	)

	// Set the ESP flag on the partition.
	log.Printf("Setting the ESP flag on %s.\n", configData.UEFI.Disk)
	utils.Execute(
		*execute,
		"parted",
		"--script",
		"--fix",
		"--align",
		"optimal",
		configData.UEFI.Disk,
		"--",
		"set",
		"1",
		"esp",
		"on",
	)

	// Sleep a few seconds to allow the partition table to update.
	if *execute {
		log.Println("Waiting...")
		time.Sleep(5 * time.Second)
	}

	// Format the UEFI partition.
	partitionNameUEFI := fmt.Sprintf("%s-part1", configData.UEFI.Disk)
	log.Printf("Formatting UEFI partition: %s\n", partitionNameUEFI)
	utils.Execute(
		*execute,
		"mkfs.vfat",
		"-n",
		"EFI",
		partitionNameUEFI,
	)

	/*
		##################################################
			ZFS Pool
		##################################################
	*/

	// Destroy any existing ZFS pool on the disks.
	log.Println("Destroying any existing ZFS pool on the disks.")
	utils.ExecuteSilent(
		*execute,
		"zpool",
		"destroy",
		"-f",
		zfsPool,
	)

	for _, zfsDisk := range configData.ZFS.Disks {
		// Clear the ZFS label on the disk.
		log.Printf("Clearing ZFS pool label on %s.\n", zfsDisk)
		utils.ExecuteSilent(
			*execute,
			"zpool",
			"labelclear",
			"-f",
			zfsDisk,
		)

		// Unmount the ZFS target device if it is mounted.
		log.Printf("Unmounting %s if it is mounted.\n", zfsDisk)
		utils.ExecuteSilent(
			*execute,
			"umount",
			zfsDisk,
		)

		// Zap the ZFS Pool disks.
		log.Printf("Zapping %s.\n", zfsDisk)
		utils.Execute(
			*execute,
			"sgdisk",
			"--zap-all",
			zfsDisk,
		)
	}

	// Run partprobe to update the partition table.
	log.Println("Running partprobe to update the partition table.")
	utils.ExecuteSilent(
		*execute,
		"partprobe",
	)

	// Sleep a few seconds to allow the partition table to update.
	if *execute {
		log.Println("Waiting...")
		time.Sleep(5 * time.Second)
	}

	// ZFS pool arguments.
	zpoolArgs := []string{"create", "-f"}

	// If compression is enabled, add the compression option.
	if configData.ZFS.Pool.Compression {
		zpoolArgs = append(zpoolArgs, "-O", "compression=zstd-3")
	}

	// If encryption is enabled, add the encryption option.
	if configData.ZFS.Pool.Encryption {
		zpoolArgs = append(zpoolArgs, "-O", "encryption=aes-256-gcm")
		zpoolArgs = append(zpoolArgs, "-O", "keyformat=passphrase")
		zpoolArgs = append(zpoolArgs, "-O", "keylocation=prompt")
	}

	// Set additional file system properties using the '-O' flag.
	zpoolArgs = append(zpoolArgs, "-O", "acltype=posixacl")
	zpoolArgs = append(zpoolArgs, "-O", "atime=off")
	zpoolArgs = append(zpoolArgs, "-O", "relatime=off")
	zpoolArgs = append(zpoolArgs, "-O", "canmount=noauto")
	zpoolArgs = append(zpoolArgs, "-O", "logbias=throughput")
	zpoolArgs = append(zpoolArgs, "-O", "mountpoint=none")
	zpoolArgs = append(zpoolArgs, "-O", "normalization=formD")
	zpoolArgs = append(zpoolArgs, "-O", "primarycache=metadata")
	zpoolArgs = append(zpoolArgs, "-O", "recordsize=32K")
	zpoolArgs = append(zpoolArgs, "-O", "secondarycache=metadata")
	zpoolArgs = append(zpoolArgs, "-O", "sync=standard")
	zpoolArgs = append(zpoolArgs, "-O", "dnodesize=auto")
	zpoolArgs = append(zpoolArgs, "-O", "xattr=sa")

	// Set additional properties, features or compatibility using the '-o' flag.
	zpoolArgs = append(zpoolArgs, "-o", "autotrim=on")
	zpoolArgs = append(zpoolArgs, "-o", "ashift=12")

	// Set the temporary mount argument.
	zpoolArgs = append(zpoolArgs, "-R", mountPoint)

	// Add the pool name to the zpool arguments.
	zpoolArgs = append(zpoolArgs, zfsPool)

	// If there is more than one root disk, we need to mirror or stripe them.
	if len(configData.ZFS.Disks) > 1 {
		if configData.ZFS.Pool.Mirror {
			log.Println("Creating mirrored ZFS pool.")
			zpoolArgs = append(zpoolArgs, "mirror")
		} else if configData.ZFS.Pool.Stripe {
			log.Println("Creating striped ZFS pool.")
		}
	}

	// Append the root disks to the zpool arguments.
	zpoolArgs = append(zpoolArgs, configData.ZFS.Disks...)

	// Create the ZFS pool.
	log.Printf("Creating ZFS pool %s.\n", zfsPool)
	utils.Execute(
		*execute,
		"zpool",
		zpoolArgs...,
	)

	/*
		##################################################
			ZFS Datasets
		##################################################
	*/

	log.Println("Creating ZFS datasets.")

	// Create the boot dataset.
	zfsDatasetPathBoot := path.Join(zfsPool, zfsDatasetBoot)
	log.Printf("Creating boot dataset: %s\n", zfsDatasetPathBoot)
	utils.Execute(
		*execute,
		"zfs",
		"create",
		"-o",
		"mountpoint=legacy",
		zfsDatasetPathBoot,
	)

	// Create the root dataset.
	zfsDatasetPathRoot := path.Join(zfsPool, zfsDatasetRoot)
	log.Printf("Creating root dataset: %s\n", zfsDatasetPathRoot)
	utils.Execute(
		*execute,
		"zfs",
		"create",
		"-o",
		"mountpoint=legacy",
		zfsDatasetPathRoot,
	)

	// Create the home dataset.
	zfsDataSetPathHome := path.Join(zfsPool, zfsDatasetHome)
	log.Printf("Creating home dataset: %s\n", zfsDataSetPathHome)
	utils.Execute(
		*execute,
		"zfs",
		"create",
		"-o",
		"mountpoint=legacy",
		zfsDataSetPathHome,
	)

	// Create the nix dataset.
	zfsDataSetPathNix := path.Join(zfsPool, zfsDatasetNixStore)
	log.Printf("Creating nix dataset: %s\n", zfsDataSetPathNix)
	utils.Execute(
		*execute,
		"zfs",
		"create",
		"-o",
		"mountpoint=legacy",
		zfsDataSetPathNix,
	)

	// Create the swap dataset.
	zfsDataSetPathSwap := path.Join(zfsPool, zfsDatasetSwap)
	log.Printf("Creating swap dataset: %s\n", zfsDataSetPathSwap)
	utils.Execute(
		*execute,
		"zfs",
		"create",
		"-V",
		configData.Swap.Size,
		zfsDataSetPathSwap,
	)

	// Create the tmp dataset.
	zfsDataSetPathTmp := path.Join(zfsPool, zfsDatasetTmp)
	log.Printf("Creating tmp dataset: %s\n", zfsDataSetPathTmp)
	utils.Execute(
		*execute,
		"zfs",
		"create",
		"-o",
		"mountpoint=legacy",
		zfsDataSetPathTmp,
	)

	// Create the var dataset.
	zfsDataSetPathVar := path.Join(zfsPool, zfsDatasetVar)
	log.Printf("Creating var dataset: %s\n", zfsDataSetPathVar)
	utils.Execute(
		*execute,
		"zfs",
		"create",
		"-o",
		"mountpoint=legacy",
		zfsDataSetPathVar,
	)

	/*
		##################################################
			Mount directories
		##################################################
	*/

	log.Println("Mounting directories.")

	// Mount the root dataset.
	log.Printf("Mounting %s to %s.\n", zfsDatasetPathRoot, mountPoint)
	utils.Execute(
		*execute,
		"mount",
		"-o",
		"X-mount.mkdir",
		"-t",
		"zfs",
		zfsDatasetPathRoot,
		mountPoint,
	)

	// Mount the boot dataset.
	log.Printf("Mounting %s to %s.\n", zfsDatasetPathBoot, mountPointBoot)
	utils.Execute(
		*execute,
		"mount",
		"-o",
		"X-mount.mkdir",
		"-t",
		"zfs",
		zfsDatasetPathBoot,
		mountPointBoot,
	)

	// Mount the UEFI partition.
	log.Printf("Mounting %s to %s.\n", partitionNameUEFI, mountPointUEFI)
	utils.Execute(
		*execute,
		"mount",
		"-t",
		"vfat",
		"-o",
		"fmask=0077,dmask=0077,iocharset=iso8859-1,X-mount.mkdir",
		partitionNameUEFI,
		mountPointUEFI,
	)

	// Mount the home dataset.
	log.Printf("Mounting %s to %s.\n", zfsDataSetPathHome, mountPointHome)
	utils.Execute(
		*execute,
		"mount",
		"-o",
		"X-mount.mkdir",
		"-t",
		"zfs",
		zfsDataSetPathHome,
		mountPointHome,
	)

	// Mount the nix dataset.
	log.Printf("Mounting %s to %s.\n", zfsDataSetPathNix, mountPointNix)
	utils.Execute(
		*execute,
		"mount",
		"-o",
		"X-mount.mkdir",
		"-t",
		"zfs",
		zfsDataSetPathNix,
		mountPointNix,
	)

	// Mount the var dataset.
	log.Printf("Mounting %s to %s.\n", zfsDataSetPathVar, mountPointVar)
	utils.Execute(
		*execute,
		"mount",
		"-o",
		"X-mount.mkdir",
		"-t",
		"zfs",
		zfsDataSetPathVar,
		mountPointVar,
	)

	// Mount the tmp dataset.
	log.Printf("Mounting %s to %s.\n", zfsDataSetPathTmp, mountPointTmp)
	utils.Execute(
		*execute,
		"mount",
		"-o",
		"X-mount.mkdir",
		"-t",
		"zfs",
		zfsDataSetPathTmp,
		mountPointTmp,
	)

	/*
		##################################################
			NixOS
		##################################################
	*/

	// Generate the NixOS configuration.
	log.Println("Generating NixOS configuration.")
	utils.Execute(
		*execute,
		"nixos-generate-config",
		"--root",
		mountPoint,
	)

	// Read the default NixOS configuration.
	if !*execute {
		log.Println("Dry run, skipping NixOS configuration modification...")
	} else {
		nixOSConfigPath := path.Join(mountPoint, "etc/nixos/configuration.nix")

		nixOSConfigDefault, err := os.ReadFile(nixOSConfigPath)
		validate.Error(err)

		// Replace the networking.hostId with the one from the config if provided.
		// Otherwise it will be generated
		var nixOSHostIdString string
		if configData.HostID != "" {
			// Use the user provided host id.
			nixOSHostIdString = configData.HostID
		} else {
			// Use the first 8 characters of the machine id.
			nixOSHostIdString = utils.ExecuteStdOut(
				*execute,
				"head",
				"-c",
				"8",
				"/etc/machine-id",
			)
		}
		regex := regexp.MustCompile("\n{\n")
		nixOSHostId := fmt.Sprintf("  networking.hostId = \"%s\";\n", nixOSHostIdString)
		nixOSConfigNew := regex.ReplaceAllString(string(nixOSConfigDefault), "\n{\n"+nixOSHostId+"\n")

		// Write the new NixOS configuration.
		err = os.WriteFile(nixOSConfigPath, []byte(nixOSConfigNew), os.ModePerm)
		validate.Error(err)
	}

	// Install NixOS.
	if *executeInstall {
		log.Println("Installing NixOS.")
		utils.Execute(
			*execute,
			"nixos-install",
			"--verbose",
			"--root",
			mountPoint,
			"--flake",
			configData.Flake,
		)
	} else {
		log.Println("You can now edit the NixOS configuration and install NixOS by running:")
		log.Printf("nixos-install --verbose --root %s --flake %s\n", mountPoint, configData.Flake)
	}

}
