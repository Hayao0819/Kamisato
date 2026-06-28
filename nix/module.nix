# NixOS services for the Kamisato daemons (ayato, miko, kayo; ayaka is a CLI).
# Binaries resolve from pkgs.<name> — add this flake's overlays.default. `settings`
# (the binary's internal/conf schema) is rendered to a config file; keep secrets in
# environmentFile (AYATO_/MIKO_/KAYO_ env), not settings (it lands in the store).
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
    mkMerge
    mkDefault
    types
    getExe
    optional
    optionalAttrs
    ;

  json = pkgs.formats.json { };
  toml = pkgs.formats.toml { };

  cfgAyato = config.services.ayato;
  cfgMiko = config.services.miko;
  cfgKayo = config.services.kayo;

  # Hardening for the plain HTTP daemons (ayato, kayo); miko is excluded (Docker).
  commonHardening = {
    NoNewPrivileges = true;
    ProtectSystem = "strict";
    ProtectHome = true;
    PrivateTmp = true;
    ProtectKernelTunables = true;
    ProtectKernelModules = true;
    ProtectControlGroups = true;
    RestrictAddressFamilies = [
      "AF_INET"
      "AF_INET6"
      "AF_UNIX"
    ];
    RestrictSUIDSGID = true;
    RestrictNamespaces = true;
    LockPersonality = true;
    SystemCallArchitectures = "native";
  };

  envFileAttrs = file: optionalAttrs (file != null) { EnvironmentFile = file; };
in
{
  options.services = {
    ayato = {
      enable = mkEnableOption "the ayato package-repository server";
      package = mkPackageOption pkgs "ayato" { };
      settings = mkOption {
        type = json.type;
        default = { };
        example = {
          port = 8080;
          store = {
            dbtype = "badgerdb";
            storagetype = "localfs";
            badgerdb = "/var/lib/ayato/badger";
            localrepodir = "/var/lib/ayato/repo";
          };
          auth.github.client_id = "Iv1.xxxx";
        };
        description = ''
          ayato_config.json contents (schema: internal/conf/ayato.go). Point local
          store paths under /var/lib/ayato (the StateDirectory); secrets go in
          environmentFile.
        '';
      };
      environmentFile = mkOption {
        type = types.nullOr types.path;
        default = null;
        description = "systemd EnvironmentFile holding AYATO_* secrets.";
      };
      openFirewall = mkOption {
        type = types.bool;
        default = false;
        description = "Open settings.port in the firewall.";
      };
    };

    miko = {
      enable = mkEnableOption "the miko build server";
      package = mkPackageOption pkgs "miko" { };
      user = mkOption {
        type = types.str;
        default = "miko";
        description = "User the service runs as.";
      };
      group = mkOption {
        type = types.str;
        default = "miko";
        description = "Primary group of the service user.";
      };
      settings = mkOption {
        type = json.type;
        default = { };
        example = {
          port = 8081;
          executor = "container";
          concurrency = 2;
          build.image = "archlinux:latest";
          ayato = {
            url = "https://repo.example.com";
            username = "miko";
          };
        };
        description = ''
          miko_config.json contents (schema: internal/conf/miko.go). The default
          executor "container" talks to the Docker daemon. Secrets (e.g. the ayato
          password via MIKO_AYATO_PASSWORD) belong in environmentFile.
        '';
      };
      environmentFile = mkOption {
        type = types.nullOr types.path;
        default = null;
        description = "systemd EnvironmentFile holding MIKO_* secrets (e.g. MIKO_AYATO_PASSWORD).";
      };
      openFirewall = mkOption {
        type = types.bool;
        default = false;
        description = "Open the miko port in the firewall.";
      };
    };

    kayo = {
      enable = mkEnableOption "the kayo AUR overlay router";
      package = mkPackageOption pkgs "kayo" { };
      settings = mkOption {
        type = toml.type;
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
  };

  config = mkMerge [

    # ---------------------------------------------------------------- ayato ----
    (mkIf cfgAyato.enable (
      let
        configFile = json.generate "ayato_config.json" cfgAyato.settings;
      in
      {
        systemd.services.ayato = {
          description = "ayato package-repository server";
          wantedBy = [ "multi-user.target" ];
          after = [ "network-online.target" ];
          wants = [ "network-online.target" ];
          serviceConfig =
            commonHardening
            // {
              ExecStart = "${getExe cfgAyato.package} --config ${configFile}";
              DynamicUser = true;
              StateDirectory = "ayato";
              Restart = "on-failure";
              RestartSec = 5;
            }
            // envFileAttrs cfgAyato.environmentFile;
        };
        networking.firewall.allowedTCPPorts = mkIf (cfgAyato.openFirewall && cfgAyato.settings ? port) [
          cfgAyato.settings.port
        ];
      }
    ))

    # ----------------------------------------------------------------- miko ----
    (mkIf cfgMiko.enable (
      let
        configFile = json.generate "miko_config.json" cfgMiko.settings;
        needsDocker = (cfgMiko.settings.executor or "container") == "container";
      in
      {
        users.users = mkIf (cfgMiko.user == "miko") {
          miko = {
            isSystemUser = true;
            group = cfgMiko.group;
            extraGroups = optional needsDocker "docker"; # container executor needs the socket
          };
        };
        users.groups = mkIf (cfgMiko.group == "miko") { miko = { }; };

        virtualisation.docker.enable = mkIf needsDocker (mkDefault true);

        systemd.services.miko = {
          description = "miko build server";
          wantedBy = [ "multi-user.target" ];
          after = [ "network-online.target" ] ++ optional needsDocker "docker.service";
          wants = [ "network-online.target" ];
          # No DynamicUser: needs the docker socket + a stable data_dir owner.
          serviceConfig = {
            ExecStart = "${getExe cfgMiko.package} --config ${configFile}";
            User = cfgMiko.user;
            Group = cfgMiko.group;
            StateDirectory = "miko";
            Restart = "on-failure";
            RestartSec = 5;
            NoNewPrivileges = true;
            PrivateTmp = true;
          }
          // envFileAttrs cfgMiko.environmentFile;
        };
        networking.firewall.allowedTCPPorts = mkIf cfgMiko.openFirewall [
          (cfgMiko.settings.port or 8081)
        ];
      }
    ))

    # ----------------------------------------------------------------- kayo ----
    (mkIf cfgKayo.enable (
      let
        # cache_dir/trust_store are forced to the durable, systemd-managed paths.
        configFile = toml.generate "kayo_config.toml" (
          cfgKayo.settings
          // {
            cache_dir = cfgKayo.cacheDir;
            trust_store = cfgKayo.trustStore;
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
            commonHardening
            // {
              ExecStart = "${getExe cfgKayo.package} --config ${configFile}";
              DynamicUser = true;
              StateDirectory = "kayo"; # /var/lib/kayo: trust.json + known_ayato.json
              CacheDirectory = "kayo"; # /var/cache/kayo: overlay clones + served repos
              Restart = "on-failure";
              RestartSec = 5;
            }
            // envFileAttrs cfgKayo.environmentFile;
        };
        networking.firewall.allowedTCPPorts = mkIf cfgKayo.openFirewall [
          (cfgKayo.settings.port or 10713)
        ];
      }
    ))
  ];
}
