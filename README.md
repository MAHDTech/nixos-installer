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

3. Copy the starting example configuration file (see configs folder for more examples)

```bash
export CONFIG_FILE="/tmp/config.yaml"

cp configs/example.yaml "${CONFIG_FILE}"
```

4. Edit the configuration file as required

```bash
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

6. Or, run the installer (go version)

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
