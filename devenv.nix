{ pkgs, ... }:
{
  env.PROJECT = "nixos-installer";

  packages = with pkgs; [
    figlet
    git
    hello
    go
    go-tools
    golangci-lint
  ];

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

  enterShell = ''
    figlet "$PROJECT"

    hello --greeting="Welcome to the $PROJECT project!"
  '';

  enterTest = ''
    echo "Running tests"
    git --version | grep --color=auto "${pkgs.git.version}"
  '';
}
