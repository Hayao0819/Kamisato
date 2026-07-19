import { describe, expect, it } from "vitest";
import { isPackageArchive, packageFileAccept } from "./package-artifact";

const suffixes = [".pkg.tar.zst", ".pkg.tar.xz", ".pkg.tar"];

describe("package artifact capabilities", () => {
    it("validates against the server-provided suffixes", () => {
        expect(isPackageArchive("foo.pkg.tar.zst", suffixes)).toBe(true);
        expect(isPackageArchive("foo.pkg.tar", suffixes)).toBe(true);
        expect(isPackageArchive("foo.pkg.tar.zip", suffixes)).toBe(false);
        expect(isPackageArchive(".pkg.tar.zst", suffixes)).toBe(false);
    });

    it("fails open when capabilities are unavailable", () => {
        expect(isPackageArchive("package.custom", [])).toBe(true);
        expect(packageFileAccept([])).toBeUndefined();
    });

    it("builds the native file input accept value", () => {
        expect(packageFileAccept(suffixes)).toBe(
            ".pkg.tar.zst,.pkg.tar.xz,.pkg.tar",
        );
    });
});
