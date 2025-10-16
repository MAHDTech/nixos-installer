{
  pkgs,
  config,
  lib,
  ...
}:
let

  # Variables
  name = "nixos-installer";
  version = "0.1.5";

  # Custom packages.
  nixos-installer = import ./devenv/nixos-installer.nix {
    inherit name;
    inherit version;

    inherit lib;
    inherit pkgs;
  };

  # Common stable packages.
  commonPackages = with pkgs; [
    figlet
    hello

    # Custom packages.
    nixos-installer.default.normal
  ];

  # Development packages.
  devPackages = with pkgs; [
    gh
    git
    go-tools
    golangci-lint
    gomod2nix
    tree
    trivy
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
      "src/vendor/"
    ];
    hooks = {
      beautysh.enable = false;
      actionlint.enable = true;
      action-validator.enable = true;
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
      staticcheck.enable = false;
      staticcheck-custom = {
        enable = true;
        entry = "staticcheck-custom";
      };
      statix.enable = true;
      trufflehog.enable = true;
      typos.enable = true;
      yamllint = {
        enable = true;
        settings = {
          configPath = ".linters/config/.yamllint.yml";
        };
      };
      go-be-lazy = {
        enable = true;
        name = "go-be-lazy";
        entry = "go-be-lazy";
        files = "^src/.*\\.*$";
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

    staticcheck-custom = {
      package = pkgs.bash;
      description = "Runs staticcheck from the src directory";
      exec = ''
        ERR=0
        # Filter files to only include .go files starting with src/
        GO_FILES=$(echo "$@" | xargs -n1 | grep "^src/.*\.go$" || true)
        if [[ -n "$GO_FILES" ]]; then
          echo "Processing Go files: $GO_FILES"
          # Extract unique directories and convert src/pkg/installer -> ./pkg/installer
          DIRS=$(echo "$GO_FILES" | xargs -n1 dirname | sed 's|^src/|./|; s|^src$|.|' | sort -u)
          pushd src > /dev/null
          for DIR in $DIRS;
          do
            echo "Checking directory: $DIR"
            staticcheck "$DIR"
            CODE="$?"
            if [[ "$CODE" -ne 0 ]];
            then
              ERR=1
            fi
          done
          popd > /dev/null
        else
          echo "No Go source files found"
        fi
        exit $ERR
      '';
    };

    go-be-lazy = {
      package = pkgs.bash;
      description = "Runs all the go commands I frequently forget";
      exec = ''
        pushd src > /dev/null
        go get -u ./... || { echo "Failed to run go get -u ./...!" ; exit 1; }
        go mod tidy || { echo "Failed to run go mod tidy!" ; exit 1; }
        go mod verify || { echo "Failed to run go mod verify!" ; exit 1; }
        go mod vendor || { echo "Failed to run go mod vendor!" ; exit 1; }
        popd > /dev/null
        gomod2nix-generate || { echo "Failed to run gomod2nix-generate!" ; exit 1; }
      '';
    };

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
