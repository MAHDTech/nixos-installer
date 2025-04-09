{
  pkgs,
  config,
  lib,
  inputs,
  ...
}:
let

  # All available unstable packages.
  pkgsUnstable = import inputs.nixpkgs-unstable {
    config.allowUnfree = true;
  };

  # Installed unstable packages.
  unstablePackages = with pkgsUnstable; [
    golangci-lint
  ];

  # Common stable packages.
  commonPackages = with pkgs; [
    figlet
    hello
  ];

  # Development packages.
  devPackages = with pkgs; [
    git
    go-tools
    #golangci-lint
  ];

in
{

  name = "nixos installer";

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

  languages.go.enable = true;
  languages.nix.enable = true;

  git-hooks = {
    excludes = [
      ".cache"
      ".devenv"
      ".direnv"
      "vendor"
    ];
    hooks = {
      actionlint.enable = true;
      beautysh.enable = true;
      check-merge-conflicts.enable = true;
      check-shebang-scripts-are-executable.enable = true;
      check-symlinks.enable = true;
      check-yaml.enable = true;
      commitizen.enable = true;
      convco.enable = true;
      gofmt.enable = true;
      golangci-lint.enable = true;
      golines.enable = true;
      gotest.enable = true;
      govet.enable = true;
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
      yamllint.enable = true;
    };
  };

  starship.enable = true;

  enterTest = ''
    echo "Running tests"
    git --version | grep --color=auto "${pkgs.git.version}"
  '';
}
