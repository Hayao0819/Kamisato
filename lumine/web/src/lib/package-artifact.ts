// The suffix list comes from ayato's capabilities response; the client contains
// no independent pacman format allowlist.
export function isPackageArchive(
    filename: string,
    supportedSuffixes: readonly string[],
): boolean {
    if (supportedSuffixes.length === 0) return true;
    return supportedSuffixes.some(
        (suffix) =>
            suffix.startsWith(".pkg.tar") &&
            filename.length > suffix.length &&
            filename.endsWith(suffix),
    );
}

export function packageFileAccept(
    supportedSuffixes: readonly string[],
): string | undefined {
    return supportedSuffixes.length > 0
        ? supportedSuffixes.join(",")
        : undefined;
}
