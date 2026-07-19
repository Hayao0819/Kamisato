# kayo AUR overlay router (NixOS service). See ../module.nix for the
# secrets/settings convention shared by all Kamisato services.
{
  config,
  lib,
  pkgs,
  ...
}:
let
  inherit (lib)
    mkEnableOption
    mkPackageOption
    mkOption
    mkIf
    types
    getExe
    ;
  shared = import ./_lib.nix { inherit lib; };
  toml = pkgs.formats.toml { };
  cfg = config.services.kayo;
in
{
  options.services.kayo = {
    enable = mkEnableOption "the kayo AUR overlay router";
    package = mkPackageOption pkgs "kayo" { };
    settings = mkOption {
      inherit (toml) type;
      default = { };
      example = {
        port = 10713;
        bind = "127.0.0.1";
        overlays = [
          {
            name = "mine";
            url = "https://git.example.com/overlay.git";
          }
        ];
        upstream.enabled = true;
      };
      description = ''
        kayo_config.toml contents (schema: internal/conf/kayo.go). cache_dir and
        trust_store are injected from the cacheDir/trustStore options; LLM API keys
        go in environmentFile.
      '';
    };
    cacheDir = mkOption {
      type = types.path;
      default = "/var/cache/kayo";
      description = "Writable cache for overlay clones and served pinned repos (kayo cache_dir).";
    };
    trustStore = mkOption {
      type = types.path;
      default = "/var/lib/kayo/trust.json";
      description = ''
        Durable trust store path (kayo trust_store). Must persist: kayo refuses a
        temp-dir fallback when ayato federation is set. known_ayato.json sits beside it.
      '';
    };
    environmentFile = mkOption {
      type = types.nullOr types.path;
      default = null;
      description = "systemd EnvironmentFile for LLM provider API keys (when the llm advisory is enabled).";
    };
    openFirewall = mkOption {
      type = types.bool;
      default = false;
      description = "Open settings.port in the firewall.";
    };
  };

  config = mkIf cfg.enable (
    let
      # cache_dir/trust_store are forced to the durable, systemd-managed paths.
      configFile = toml.generate "kayo_config.toml" (
        cfg.settings
        // {
          cache_dir = cfg.cacheDir;
          trust_store = cfg.trustStore;
        }
      );
    in
    {
      systemd.services.kayo = {
        description = "kayo AUR overlay router";
        wantedBy = [ "multi-user.target" ];
        after = [ "network-online.target" ];
        wants = [ "network-online.target" ];
        path = [ pkgs.git ]; # kayo shells out to git to clone overlays
        serviceConfig =
          shared.commonHardening
          // {
            ExecStart = "${getExe cfg.package} --config ${configFile}";
            DynamicUser = true;
            StateDirectory = "kayo"; # /var/lib/kayo: trust.json + known_ayato.json
            CacheDirectory = "kayo"; # /var/cache/kayo: overlay clones + served repos
            Restart = "on-failure";
            RestartSec = 5;
          }
          // shared.envFileAttrs cfg.environmentFile;
      };
      networking.firewall.allowedTCPPorts = mkIf cfg.openFirewall [
        (cfg.settings.port or 10713)
      ];
    }
  );
}
