{
  pkgs,
  lib,
  config,
  inputs,
  ...
}:
let
  pkgs-unstable = import inputs.nixpkgs-unstable { system = pkgs.stdenv.system; };
in
# See full reference at https://devenv.sh/reference/options/
{
  # https://devenv.sh/packages/
  packages = [
    pkgs.gnumake
    pkgs.jq
    pkgs.shellcheck
    pkgs.osv-scanner
  ];

  # https://devenv.sh/languages/
  languages.go.enable = true;
  languages.go.version = "1.25.7";

  git-hooks.hooks = {
    gofmt.enable = true;
    golangci-lint.enable = true;
    osv-scanner = {
      enable = true;
      name = "osv-scanner";
      entry = "osv-scanner scan -r .";
      files = "\\.(mod|sum)$";
      pass_filenames = false;
    };
  };

}
