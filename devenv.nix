{
  pkgs,
  config,
  lib,
  inputs,
  ...
}:
let

  # Variables
  name = "nixos-installer";
  version = "1.0.0";

  # Custom packages.
  nixos-installer = import ./devenv/nixos-installer.nix {
    inherit name;
    inherit version;

    inherit lib;
    inherit pkgs;
  };

  # All available unstable packages.
  pkgsUnstable = import inputs.nixpkgs-unstable {
    config.allowUnfree = true;
  };

  # Installed unstable packages.
  unstablePackages = with pkgsUnstable; [
    #golangci-lint
  ];

  # Common stable packages.
  commonPackages = with pkgs; [
    figlet
    hello

    # Custom packages.
    nixos-installer.default.normal
  ];

  # Development packages.
  devPackages = with pkgs; [
    git
    go-tools
    golangci-lint
    gomod2nix
  ];

in
{

  inherit name;

  env = {
    PROJECT = config.name;
  };

  devenv = {
    warnOnNewVersion = true;
  };

  dotenv = {
    enable = true;
    disableHint = false;
  };

  difftastic = {
    enable = true;
  };

  packages =
    commonPackages
    ++ unstablePackages
    ++ lib.optionals (!config.container.isBuilding || config.name == "devenv") devPackages;

  enterShell = ''
    figlet -f starwars -w 180 $PROJECT

    hello --greeting="Hello ''${USER:-user}, welcome to the $PROJECT project!"

    echo ""
    echo "#########################"
    echo "#### Helper scripts #####"
    echo "#########################"
    echo "🦾"
    ${pkgs.gnused}/bin/sed -e 's| |••|g' -e 's|=| |' <<EOF | ${pkgs.util-linuxMinimal}/bin/column -t | ${pkgs.gnused}/bin/sed -e 's|^|🦾 |' -e 's|••| |g'
    ${lib.generators.toKeyValue { } (lib.mapAttrs (_name: value: value.description) config.scripts)}
    EOF
    echo "🦾"
    echo "#########################"
  '';

  languages = {
    go = {
      enable = true;
    };
    nix = {
      enable = true;
    };
  };

  git-hooks = {
    excludes = [
      ".cache"
      ".devenv"
      ".direnv"
      "src/vendor"
    ];
    hooks = {
      beautysh.enable = false;
      actionlint.enable = true;
      check-merge-conflicts.enable = true;
      check-shebang-scripts-are-executable.enable = true;
      check-symlinks.enable = true;
      check-yaml.enable = true;
      commitizen.enable = true;
      convco.enable = true;
      gofmt.enable = true;
      golangci-lint = {
        enable = true;
        pass_filenames = false;
      };
      golines.enable = true;
      gotest.enable = true;
      govet = {
        enable = true;
        pass_filenames = false;
      };
      gptcommit.enable = true;
      mixed-line-endings.enable = true;
      nixfmt-rfc-style.enable = true;
      prettier.enable = true;
      pretty-format-json.enable = true;
      revive.enable = true;
      ripsecrets.enable = true;
      shellcheck.enable = true;
      shfmt.enable = true;
      staticcheck.enable = true;
      statix.enable = true;
      trufflehog.enable = true;
      typos.enable = true;
      yamllint = {
        enable = true;
        settings = {
          configPath = ".linters/config/.yamllint.yml";
        };
      };
      # Custom hook to generate the gomod2nix.toml file.
      gomod2nix-generate = {
        enable = true;
        name = "gomod2nix-generate";
        entry = "gomod2nix-generate";
        files = "^src/vendor/.*\\.*$";
        pass_filenames = false;
      };
    };
  };

  starship.enable = true;

  enterTest = ''
    echo "Running tests"
    git --version | grep --color=auto "${pkgs.git.version}"
  '';

  scripts = {
    gomod2nix-generate = {
      package = pkgs.bash;
      description = "Generate the gomod2nix.toml file";
      exec = ''
        pushd src > /dev/null
        gomod2nix generate
        popd > /dev/null
      '';
    };

  };

  outputs = {
    # Default builds
    nixos-installer = nixos-installer.default.normal;

    # Platform specific builds
    nixos-installer-darwin-amd64 = nixos-installer."x86_64-darwin".normal;
    nixos-installer-darwin-arm64 = nixos-installer."aarch64-darwin".normal;
    nixos-installer-linux-amd64 = nixos-installer."x86_64-linux".normal;
    nixos-installer-linux-arm64 = nixos-installer."aarch64-linux".normal;
  };
}
