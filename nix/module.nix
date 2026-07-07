# NixOS services for the Kamisato daemons (ayato, miko, kayo; ayaka is a CLI).
# Binaries resolve from pkgs.<name> — add this flake's overlays.default. `settings`
# (the binary's internal/conf schema) is rendered to a config file; keep secrets in
# environmentFile (AYATO_/MIKO_/KAYO_ env), not settings (it lands in the store).
#
# Each service lives in its own module under ./modules; this file only imports them.
{
  imports = [
    ./modules/ayato.nix
    ./modules/miko.nix
    ./modules/kayo.nix
  ];
}
