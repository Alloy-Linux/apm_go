{
  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/nixos-24.11";
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
              alias apmt='./build/apm'
              echo "Development environment loaded"
            '';
          };
        };

        packages.default = pkgs.buildGoModule {
          pname = "apm";
          version = "0.1.0";
          src = ./.;
          vendorHash = "sha256-NUIpLPchOsLwPz0dp+jmcUmRdKOlPNVGrUq0YxkfZpQ=";
          subPackages = [ "src" ];
          
          nativeBuildInputs = with pkgs; [
            pkg-config
            sqlite
          ];
          
          
          # don't change this
          buildFlags = [ "-mod=readonly" ];
          
          postInstall = ''
            mv $out/bin/src $out/bin/apm
          '';
          
          meta = with pkgs.lib; {
            description = "Alloy Package Manager";
            homepage = "https://github.com/Alloy-Linux/apm_go";
            license = licenses.gpl3;
            maintainers = [ pkgs.lib.maintainers.Simon-Weij ];
          };
        };
      }
    );
}