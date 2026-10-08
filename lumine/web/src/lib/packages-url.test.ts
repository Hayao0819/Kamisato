import { describe, expect, it } from "vitest";
import {
    buildPackagesQuery,
    nextPackagesSort,
    type PackagesQuery,
    packagesHref,
    parsePackagesQuery,
} from "@/lib/packages-url";

const empty: PackagesQuery = {
    repo: "",
    arch: "",
    q: "",
    group: null,
    type: null,
    sort: "pkgname",
    dir: "asc",
    page: 1,
    per: 50,
};

describe("parsePackagesQuery", () => {
    it("returns defaults for an empty query", () => {
        expect(parsePackagesQuery(new URLSearchParams())).toEqual(empty);
    });

    it("reads every supported param", () => {
        const p = new URLSearchParams(
            "repo=core&arch=x86_64&q=bash&group=base&type=split&sort=size&dir=desc&page=3&per=100",
        );
        expect(parsePackagesQuery(p)).toEqual({
            repo: "core",
            arch: "x86_64",
            q: "bash",
            group: "base",
            type: "split",
            sort: "size",
            dir: "desc",
            page: 3,
            per: 100,
        });
    });

    it("clamps an invalid sort to pkgname and a non-desc dir to asc", () => {
        const p = new URLSearchParams("sort=bogus&dir=sideways");
        const q = parsePackagesQuery(p);
        expect(q.sort).toBe("pkgname");
        expect(q.dir).toBe("asc");
    });

    it("floors page at 1 and uses the default page size for junk per", () => {
        const q = parsePackagesQuery(new URLSearchParams("page=-5&per=abc"));
        expect(q.page).toBe(1);
        expect(q.per).toBe(50);
    });

    it.each([
        "-1",
        "0",
        "1.5",
        "Infinity",
        "NaN",
        "9007199254740992",
    ])("rejects invalid pagination values %s", (value) => {
        const query = parsePackagesQuery(
            new URLSearchParams({ page: value, per: value }),
        );
        expect(query.page).toBe(1);
        expect(query.per).toBe(50);
    });

    it("preserves valid positive custom page sizes", () => {
        expect(
            parsePackagesQuery(new URLSearchParams("page=2&per=20")),
        ).toMatchObject({ page: 2, per: 20 });
    });
});

describe("nextPackagesSort", () => {
    it("changes both key and direction in one URL update", () => {
        const next = {
            ...empty,
            ...nextPackagesSort(empty, "size"),
        };
        const params = new URLSearchParams(buildPackagesQuery(next));
        expect(params.get("sort")).toBe("size");
        expect(params.get("dir")).toBe("desc");
        expect(nextPackagesSort(next, "pkgver")).toEqual({
            sort: "pkgver",
            dir: "asc",
        });
    });

    it("toggles only direction for the active column", () => {
        expect(nextPackagesSort(empty, "pkgname")).toEqual({
            sort: "pkgname",
            dir: "desc",
        });
        expect(nextPackagesSort({ sort: "size", dir: "desc" }, "size")).toEqual(
            { sort: "size", dir: "asc" },
        );
    });
});

describe("buildPackagesQuery", () => {
    it("omits defaults and unset params", () => {
        expect(buildPackagesQuery(empty)).toBe("");
    });

    it("serializes only non-default values", () => {
        const qs = buildPackagesQuery({
            ...empty,
            repo: "core",
            sort: "size",
            dir: "desc",
            page: 2,
            per: 100,
        });
        const out = new URLSearchParams(qs);
        expect(out.get("repo")).toBe("core");
        expect(out.get("sort")).toBe("size");
        expect(out.get("dir")).toBe("desc");
        expect(out.get("page")).toBe("2");
        expect(out.get("per")).toBe("100");
        expect(out.get("arch")).toBeNull();
    });

    it("round-trips through parse", () => {
        const q: PackagesQuery = {
            ...empty,
            repo: "extra",
            arch: "aarch64",
            q: "vim",
            group: "base-devel",
            type: "pkg",
            sort: "builddate",
            dir: "desc",
            page: 4,
            per: 250,
        };
        expect(
            parsePackagesQuery(new URLSearchParams(buildPackagesQuery(q))),
        ).toEqual(q);
    });
});

describe("packagesHref", () => {
    it("returns the bare path when there is no query", () => {
        expect(packagesHref(empty)).toBe("/packages");
    });

    it("appends the query string when present", () => {
        expect(packagesHref({ ...empty, repo: "core" })).toBe(
            "/packages?repo=core",
        );
    });
});
