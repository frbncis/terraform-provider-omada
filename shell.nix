# Dev shell for terraform-provider-omada.
#
# Pins the toolchain to the versions the repo expects (.tool-versions): Go
# 1.26.5 and golangci-lint 2.12.2. Terraform is intentionally NOT provided
# here — it is unfree in nixpkgs and builds from source slowly, and the
# provider is driven from the homelab-infra dev shell (which already ships
# terraform via flake.nix).
#
# Usage:
#   nix-shell
#   make build    # go build -> ./terraform-provider-omada

{ pkgs ? import <nixpkgs> { } }:

with pkgs;

mkShell {
  buildInputs = [
    go            # 1.26.5
    golangci-lint # 2.12.2

    jq
  ];

  shellHook = ''
    echo "go:            $(go version)"
    echo "golangci-lint: $(golangci-lint version 2>/dev/null | head -n1)"
  '';
}
