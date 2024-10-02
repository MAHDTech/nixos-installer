{ pkgs, lib, config, inputs, ... }:

{
  env.PROJECT = "nix-installer";

  packages = with pkgs; [
    figlet
    git
    hello
  ];

  languages.go.enable = true;

  enterShell = ''
    figlet "$PROJECT"

    hello --greeting="Welcome to the $PROJECT project!"
  '';

  enterTest = ''
    echo "Running tests"
    git --version | grep --color=auto "${pkgs.git.version}"
  '';

  pre-commit.hooks.shellcheck.enable = true;

}
