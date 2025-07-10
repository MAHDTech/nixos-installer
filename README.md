# nix-installer

Installs NixOS using ZFS in an opinionated way.

## Background

This installs NixOS;

- Using a dedicated UEFI drive.
- Uses entire disk or disks for ZFS as the root filesystem with optional stripe or mirror.
- Configures common mount paths as ZFS datasets
- Configures the system to use the specified flake

## Usage

1. Boot the NixOS Live **minimal** ISO. (The graphical installer no longer includes ZFS support.)

2. Setup Networking

3. Define your configuration file.

```bash
# Option 1: Use a predefined config from GitHub (configs/HYPERVISOR-1.yaml)
export CONFIG_FILE="HYPERVISOR-1"

# Option 2: Use a local config file (e.g. /tmp/config.yaml)
export CONFIG_FILE="/tmp/config.yaml"

# Edit the file to meet your needs.
cp configs/example.yaml "${CONFIG_FILE}"
vim "${CONFIG_FILE}"
```

4. Run the installer (nix version)

```bash
# Dry run
sudo nix \
    --extra-experimental-features nix-command \
    --extra-experimental-features flakes \
    run github:MAHDTech/nixos-installer \
    -- \
        -config "${CONFIG_FILE}"

# Nuke all the things.
sudo nix \
    --extra-experimental-features nix-command \
    --extra-experimental-features flakes \
    run github:MAHDTech/nixos-installer \
    -- \
        -config "${CONFIG_FILE}" \
        -run
```

5. Or, run the installer (go version)

```bash
nix-shell -p git go

git clone git@github.com:MAHDTech/nixos-installer.git

cd nixos-installer

# Dry run
sudo -E go run main.go \
  -config "${CONFIG_FILE}"

# Nuke all the things but don't auto-install
sudo -E go run main.go \
  -config "${CONFIG_FILE}" \
  -run

# Nuke all the things and auto-install configured flake.
sudo -E go run main.go \
  -config "${CONFIG_FILE}" \
  -run \
  -install
```

## Configuration Options

The installer supports two ways to specify configuration files:

### Config Names (Automatic GitHub Fetch)

When you pass a simple name without path separators or file extensions:

- `HYPERVISOR-1` → fetches `configs/HYPERVISOR-1.yaml` from GitHub
- `example` → fetches `configs/example.yaml` from GitHub

**Important**: The config is automatically fetched from the **same branch/ref** that you're running the installer from:

```bash
# Fetches config from main branch
sudo nix \
    --extra-experimental-features nix-command \
    --extra-experimental-features flakes \
    run github:MAHDTech/nixos-installer \
    -- \
        -config HYPERVISOR-1

# Fetches config from my-feature-branch
sudo nix \
    --extra-experimental-features nix-command \
    --extra-experimental-features flakes \
    run github:MAHDTech/nixos-installer/my-feature-branch \
    -- \
        -config HYPERVISOR-1

# Fetches config from specific commit
sudo nix \
    --extra-experimental-features nix-command \
    --extra-experimental-features flakes \
    run github:MAHDTech/nixos-installer/abc123def \
    -- \
        -config HYPERVISOR-1
```

This ensures that the configuration always matches the version of the installer you're running.

### File Paths (Local Files)

When you pass a path with separators or file extensions:

- `./config.yaml` → reads local file
- `/tmp/my-config.yaml` → reads local file
- `configs/HYPERVISOR-1.yaml` → reads local file (if cloned repo)
