{
  description = "Reproducible shell for the BIP157/BIP158 conformance suite";

  inputs.nixpkgs.url = "github:NixOS/nixpkgs/nixpkgs-unstable";
  inputs.kyoto-src = {
    url = "github:2140-dev/kyoto/ae6b20f721a45cfc06d29d5cf03c8e5ac8148e50";
    flake = false;
  };
  inputs.neutrino-src = {
    url = "github:lightninglabs/neutrino/v0.17.1";
    flake = false;
  };
  inputs.nakamoto-src = {
    url = "github:cloudhead/nakamoto/76ab7a3b6207373399cd15a90037294bb08beeb5";
    flake = false;
  };
  inputs.wasabi-src = {
    url = "github:WalletWasabi/WalletWasabi/3660452fa4655b8af0bedd33ceb44948d53ee660";
    flake = false;
  };
  inputs.chutney-src = {
    url = "git+https://gitlab.torproject.org/tpo/core/chutney.git?rev=3c757c2b9d14e8d72e4bf2e65ce2cd08f86380b9";
    flake = false;
  };

  outputs = { self, nixpkgs, kyoto-src, neutrino-src, nakamoto-src, wasabi-src, chutney-src }:
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
              pkgs.cjdns
              pkgs.dotnet-sdk_10
              pkgs.go
              pkgs.gotestsum
              pkgs.i2p
              pkgs.i2pd
              pkgs.iproute2
              pkgs.jq
              pkgs.just
              pkgs.openjdk_headless
              pkgs.protobuf
              pkgs.rustc
              pkgs.tor
            ];
            shellHook = ''
              export KYOTO_SOURCE=${kyoto-src}
              export NEUTRINO_SOURCE=${neutrino-src}
              export NAKAMOTO_SOURCE=${nakamoto-src}
              export WASABI_SOURCE=${wasabi-src}
              export CHUTNEY_SOURCE=${chutney-src}
              export WASABI_P2P_PATCH=$PWD/nix/patches/wasabi/0001-p2p-compact-filter-provider.patch
            '';
          };
        });
    };
}
