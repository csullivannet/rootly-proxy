{
  description = "Go development environment for rootly-proxy";

  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/nixos-unstable";
    flake-utils.url = "github:numtide/flake-utils";
  };

  outputs = { self, nixpkgs, flake-utils }:
    flake-utils.lib.eachDefaultSystem (system:
      let
        pkgs = nixpkgs.legacyPackages.${system};
      in
      {
        devShells.default = pkgs.mkShell {
          buildInputs = with pkgs; [
            # Go development
            go
            gopls
            gotools
            go-tools

            # Database
            postgresql

            # Development tools
            git
            curl
            jq
            go-task

            # Optional: Docker for containerization
            docker
            docker-compose

            # Testing and debugging
            delve

            # TLS/Certificate tools
            openssl
          ];

          shellHook = ''
            echo "Go development environment loaded"
            echo "Go version: $(go version)"
            echo ""
            echo "Available commands:"
            echo "  go run main.go    - Run the application"
            echo "  go build          - Build the application"
            echo "  go test ./...     - Run tests"
            echo "  go fmt ./...      - Format code"
            echo "  go vet ./...      - Vet code"
            echo "  task              - Run tasks defined in Taskfile.yaml"
            echo ""

            # Set GOPATH and GOBIN if not already set
            export GOPATH="''${GOPATH:-$HOME/go}"
            export GOBIN="''${GOBIN:-$GOPATH/bin}"
            export PATH="$GOBIN:$PATH"
          '';
        };
      });
}