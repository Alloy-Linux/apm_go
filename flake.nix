{
  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/nixos-unstable";
    flake-utils.url = "github:numtide/flake-utils";
  };

  outputs = { self, nixpkgs, flake-utils, ... }:
    flake-utils.lib.eachDefaultSystem (system:
      let
        pkgs = import nixpkgs { inherit system; };
      in {
        devShells = {
          default = pkgs.mkShell {
            packages = [
              pkgs.go
              pkgs.gopls
              pkgs.delve
              pkgs.sqlite
            ];
            shellHook = ''
              alias build='mkdir -p build && cd build && go build -o apm ../src && cd ..'
              alias run='mkdir -p build && cd build && go build -o apm .. && ./apm; cd ..'
              alias apm='./build/apm'
              echo "Development environment loaded"
            '';
          };
        };

        defaultPackage = pkgs.buildGoModule {
          pname = "apm";
          version = "0.0.1";
          src = ./.;
          vendorHash = null;
          subPackages = [ "src" ];
          CGO_ENABLED = 1;
          buildFlagsArray = [ "-o=build/flk" ];
          installPhase = ''
            mkdir -p $out/bin
            cp build/flk $out/bin/
          '';
        };
      }
    );
}