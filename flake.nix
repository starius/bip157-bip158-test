{
  description = "Reproducible shell for the BIP157/BIP158 conformance suite";

  inputs.nixpkgs.url = "github:NixOS/nixpkgs/nixpkgs-unstable";

  outputs = { self, nixpkgs }:
    let
      systems = [ "x86_64-linux" "aarch64-linux" ];
      forAllSystems = nixpkgs.lib.genAttrs systems;
    in
    {
      devShells = forAllSystems (system:
        let
          pkgs = import nixpkgs { inherit system; };
        in
        {
          default = pkgs.mkShell {
            buildInputs = [
              pkgs.bitcoind
              pkgs.cargo
              pkgs.dotnet-sdk_10
              pkgs.go
              pkgs.gotestsum
              pkgs.iproute2
              pkgs.jq
              pkgs.just
              pkgs.protobuf
              pkgs.rustc
            ];
          };
        });
    };
}
