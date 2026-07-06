package keyring

import "fmt"

// installScript renders the .INSTALL hook. post_install/post_upgrade run
// `pacman-key --populate <name>`, which imports <name>.gpg, lsigns the -trusted
// fingerprints and disables the -revoked ones. The pacman-key presence guard
// mirrors the archlinux-keyring/chaotic install scripts so the hook is a no-op on
// a system whose keyring is not yet initialised.
func installScript(name string) string {
	return fmt.Sprintf(`post_upgrade() {
	if usr/bin/pacman-key -l >/dev/null 2>&1; then
		usr/bin/pacman-key --populate %[1]s
	else
		echo ">>> Run 'pacman-key --init' then 'pacman-key --populate %[1]s' to trust this keyring."
	fi
}

post_install() {
	if [ -x usr/bin/pacman-key ]; then
		post_upgrade
	fi
}
`, name)
}
