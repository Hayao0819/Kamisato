import { describe, expect, it } from "vitest";
import { APIClient } from "@/lib/api";

const BASE = "https://ayato.example";

function client() {
    return new APIClient({
        AYATO_URL: BASE,
        AUTH_MODE: "cookie",
        FALLBACK: false,
    });
}

describe("APIEndpoints", () => {
    const e = client().endpoints;
    const api = `${BASE}/api/unstable`;

    it("builds the unstable API root from the server URL", () => {
        expect(e.apiUnstableUrl).toBe(api);
    });

    it("builds package endpoints", () => {
        expect(e.allPkgs("core", "x86_64")).toBe(`${api}/core/x86_64/package`);
        expect(e.packageDetail("core", "x86_64", "bash")).toBe(
            `${api}/core/x86_64/package/bash`,
        );
        expect(e.uploadPackage("core")).toBe(`${api}/core/package`);
    });

    it("builds repo and arch endpoints", () => {
        expect(e.repos()).toBe(`${api}/repos`);
        expect(e.arches("core")).toBe(`${api}/repos/core/archs`);
        expect(e.repoFile("core", "x86_64", "core.db")).toBe(
            `${BASE}/repo/core/x86_64/core.db`,
        );
    });

    it("builds auth and hello endpoints", () => {
        expect(e.hello()).toBe(`${api}/hello`);
        expect(e.authMe()).toBe(`${api}/auth/me`);
    });

    it("builds build/job endpoints", () => {
        expect(e.submitBuild()).toBe(`${api}/build`);
        expect(e.listJobs()).toBe(`${api}/jobs`);
        expect(e.jobDetail("j1")).toBe(`${api}/jobs/j1`);
        expect(e.jobLogs("j1")).toBe(`${api}/jobs/j1/logs`);
        expect(e.cancelJob("j1")).toBe(`${api}/jobs/j1`);
        expect(e.stats()).toBe(`${api}/stats`);
    });
});

describe("APIClient runtime config", () => {
    it("exposes the configured server URL and is executable", () => {
        const api = client();
        expect(api.serverUrl).toBe(BASE);
        expect(api.endpoints.executable).toBe(true);
    });

    it("falls back to a null server URL that is not executable", () => {
        const api = APIClient.fallback();
        expect(api.serverUrl).toBeNull();
        expect(api.endpoints.executable).toBe(false);
    });
});
