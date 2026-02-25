# nix-installer

Installs NixOS using ZFS in an opinionated way.

_Archived — replaced by the amazing disko for declarative disk partitioning._

## Background

This installs NixOS from a configuration file;

- Using a dedicated UEFI drive.
- Uses entire disk or disks for ZFS as the root filesystem with optional stripe, mirror or raidz.
- Configures common mount paths as ZFS datasets
- Configures the system to use the specified flake

## Usage

1. Boot the NixOS Live ISO (be sure to boot an LTS kernel for ZFS support).

2. Setup Networking

3. Either download a release binary or run the installer using the one-shot script:

```bash
# Dry run example using a remote config
curl -fsSL https://raw.githubusercontent.com/MAHDTech/nixos-installer/trunk/scripts/yolo.sh | sudo bash -s -- -config HYPERVISOR-1

# Execute mode example using a local config
curl -fsSL https://raw.githubusercontent.com/MAHDTech/nixos-installer/trunk/scripts/yolo.sh | sudo bash -s -- -config /tmp/config.yaml -run

# Execute mode example that also installs the defined Nix flake.
curl -fsSL https://raw.githubusercontent.com/MAHDTech/nixos-installer/trunk/scripts/yolo.sh | sudo bash -s -- -config HYPERVISOR-1 -run -install
```

## Configuration Options

The installer supports two ways to specify configuration files:

### Config Names (Automatic GitHub Fetch)

When you pass a simple name without path separators or file extensions:

- `HYPERVISOR-1` → fetches `configs/HYPERVISOR-1.yaml` from GitHub
- `example` → fetches `configs/example.yaml` from GitHub

### File Paths (Local Files)

When you pass a path with separators or file extensions:

- `./config.yaml` → reads local file
- `/tmp/my-config.yaml` → reads local file
- `configs/HYPERVISOR-1.yaml` → reads local file (if cloned repo)
