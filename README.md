# nix-installer

Installs NixOS using ZFS in an opinionated way.

## Background

This installs NixOS;

- Using a dedicated UEFI drive.
- Uses entire disk or disks for ZFS as the root filesystem with optional stripe or mirror.
- Configures common mount paths as ZFS datasets
- Configures the system to use the specified flake

## Usage

1. Boot the NixOS Live ISO

2. Setup Networking

3. Create a YAML configuration file (see config.yaml for an example)

```bash
vim /tmp/config.yaml
```

4. Run the installer

```bash
# Dry run
nix \
    --extra-experimental-features nix-command \
    --extra-experimental-features flakes \
    run github:MAHDTech/nixos-installer \
    -- \
        -config /tmp/config.yaml

# Nuke all the things.
nix \
    --extra-experimental-features nix-command \
    --extra-experimental-features flakes \
    run github:MAHDTech/nixos-installer \
    -- \
        -config /tmp/config.yaml \
        -run
```
