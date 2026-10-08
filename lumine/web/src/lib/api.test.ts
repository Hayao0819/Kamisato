import { afterEach, describe, expect, it, vi } from "vitest";
import { APIClient, type BugReportInput } from "@/lib/api";

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
        expect(e.allPkgs("core", "x86_64")).toBe(
            `${api}/repos/core/x86_64/packages`,
        );
        expect(e.packageDetail("core", "x86_64", "bash")).toBe(
            `${api}/repos/core/x86_64/packages/bash`,
        );
        expect(e.uploadPackage("core")).toBe(`${api}/repos/core/packages`);
    });

    it("builds repo and arch endpoints", () => {
        expect(e.repos()).toBe(`${api}/repos`);
        expect(e.arches("core")).toBe(`${api}/repos/core`);
        expect(e.repoFile("core", "x86_64", "core.db")).toBe(
            `${BASE}/repo/core/x86_64/core.db`,
        );
    });

    it("builds auth and hello endpoints", () => {
        expect(e.hello()).toBe(`${api}/hello`);
        expect(e.authMe()).toBe(`${api}/auth/me`);
    });

    it("builds feature and bug-report endpoints", () => {
        expect(e.features()).toBe(`${api}/features`);
        expect(e.bugReports()).toBe(`${api}/bug-reports`);
    });

    it("builds build/job endpoints", () => {
        expect(e.submitBuild()).toBe(`${api}/build`);
        expect(e.listJobs()).toBe(`${api}/jobs`);
        expect(e.jobDetail("j1")).toBe(`${api}/jobs/j1`);
        expect(e.jobLogs("j1")).toBe(`${api}/jobs/j1/logs`);
        expect(e.jobLogToken("j1")).toBe(`${api}/jobs/j1/logs/token`);
        expect(e.cancelJob("j1")).toBe(`${api}/jobs/j1`);
        expect(e.stats()).toBe(`${api}/stats`);
    });

    it("encodes every dynamic value as one path segment", () => {
        expect(e.packageDetail("tier/core", "x86 64", "pkg/name")).toBe(
            `${api}/repos/tier%2Fcore/x86%2064/packages/pkg%2Fname`,
        );
        expect(e.jobLogs("job/with space")).toBe(
            `${api}/jobs/job%2Fwith%20space/logs`,
        );
    });

    it("normalizes a prefixed static runtime base with a trailing slash", () => {
        const apiClient = new APIClient({
            AYATO_URL: `${BASE}/prefix/`,
            AUTH_MODE: "bearer",
            FALLBACK: false,
        });
        expect(apiClient.serverUrl).toBe(`${BASE}/prefix`);
        expect(apiClient.endpoints.apiUnstableUrl).toBe(
            `${BASE}/prefix/api/unstable`,
        );
    });
});

describe("APIClient log authentication", () => {
    afterEach(() => {
        vi.restoreAllMocks();
        vi.unstubAllGlobals();
    });

    it("mints a one-time token with cookie credentials and propagates cancellation", async () => {
        const fetchMock = vi.fn<FetchFn>(
            async () => new Response(JSON.stringify({ token: "once/+?" })),
        );
        vi.stubGlobal("fetch", fetchMock);
        const controller = new AbortController();

        const url = await client().jobLogsUrl("job/one", controller.signal);

        expect(fetchMock.mock.calls[0][0]).toBe(
            `${BASE}/api/unstable/jobs/job%2Fone/logs/token`,
        );
        expect(fetchMock.mock.calls[0][1]).toMatchObject({
            method: "POST",
            credentials: "include",
            signal: controller.signal,
        });
        expect(url).toBe(
            `${BASE}/api/unstable/jobs/job%2Fone/logs?token=once%2F%2B%3F`,
        );
    });

    it("uses the bearer auth strategy but never places the session bearer in the URL", async () => {
        const api = new APIClient({
            AYATO_URL: BASE,
            AUTH_MODE: "bearer",
            FALLBACK: false,
        });
        const decorate = vi
            .spyOn(api.auth, "decorate")
            .mockImplementation((init) => ({
                ...init,
                headers: { Authorization: "Bearer session-secret" },
                credentials: "omit",
            }));
        const fetchMock = vi.fn<FetchFn>(
            async () => new Response(JSON.stringify({ token: "one-time" })),
        );
        vi.stubGlobal("fetch", fetchMock);

        const url = await api.jobLogsUrl("job1");

        expect(decorate).toHaveBeenCalledOnce();
        expect(fetchMock.mock.calls[0][1]).toMatchObject({
            credentials: "omit",
            headers: { Authorization: "Bearer session-secret" },
        });
        expect(url).toBe(`${BASE}/api/unstable/jobs/job1/logs?token=one-time`);
        expect(url).not.toContain("session-secret");
    });

    it("does not open an unauthenticated stream when token minting fails", async () => {
        vi.stubGlobal(
            "fetch",
            vi.fn(async () => new Response("denied", { status: 401 })),
        );
        await expect(client().jobLogsUrl("job1")).rejects.toThrow(
            "ログインが必要です",
        );
    });

    it.each([
        {},
        { token: "" },
        { token: 123 },
    ])("rejects an invalid token response %j", async (payload) => {
        vi.stubGlobal(
            "fetch",
            vi.fn(async () => new Response(JSON.stringify(payload))),
        );
        await expect(client().jobLogsUrl("job1")).rejects.toThrow("トークン");
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

type FetchFn = (
    input: RequestInfo | URL,
    init?: RequestInit,
) => Promise<Response>;

describe("APIClient feature and bug-report calls", () => {
    afterEach(() => {
        vi.unstubAllGlobals();
    });

    it("fetches the features object from the features endpoint", async () => {
        const payload = {
            bug_report: true,
            miko: false,
            github_login: true,
            recaptcha_site_key: "site-key",
            package_archive_suffixes: [".pkg.tar.zst", ".pkg.tar"],
        };
        const fetchMock = vi.fn<FetchFn>(
            async () => new Response(JSON.stringify(payload), { status: 200 }),
        );
        vi.stubGlobal("fetch", fetchMock);

        const features = await client().features();

        expect(fetchMock).toHaveBeenCalledTimes(1);
        expect(fetchMock.mock.calls[0][0]).toBe(
            `${BASE}/api/unstable/features`,
        );
        expect(features).toEqual(payload);
    });

    it("posts a bug report and returns the created issue url", async () => {
        const fetchMock = vi.fn<FetchFn>(
            async () =>
                new Response(
                    JSON.stringify({ url: "https://issues.example/1" }),
                    { status: 201 },
                ),
        );
        vi.stubGlobal("fetch", fetchMock);

        const input: BugReportInput = {
            pkgname: "bash",
            pkgver: "5.2.0-1",
            repo: "core",
            arch: "x86_64",
            name: "reporter",
            email: "reporter@example.com",
            severity: "high",
            description: "it crashes",
            recaptcha_token: "token-123",
        };
        const res = await client().submitBugReport(input);

        const [url, init] = fetchMock.mock.calls[0];
        expect(url).toBe(`${BASE}/api/unstable/bug-reports`);
        expect(init?.method).toBe("POST");
        const body = JSON.parse(init?.body as string);
        expect(body).toEqual(input);
        expect(body.repo).toBe("core");
        expect(body.arch).toBe("x86_64");
        expect(res).toEqual({ url: "https://issues.example/1" });
    });

    it("throws when the bug-report submit fails", async () => {
        const fetchMock = vi.fn(
            async () => new Response("nope", { status: 400 }),
        );
        vi.stubGlobal("fetch", fetchMock);

        await expect(
            client().submitBugReport({
                pkgname: "bash",
                pkgver: "5.2.0-1",
                name: "",
                email: "",
                severity: "medium",
                description: "x",
                recaptcha_token: "",
            }),
        ).rejects.toThrow();
    });
});
