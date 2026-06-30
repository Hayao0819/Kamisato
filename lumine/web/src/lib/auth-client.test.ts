import { afterEach, describe, expect, it, vi } from "vitest";
import { createAuthClient } from "@/lib/auth-client";

const BASE = "https://ayato.example";

afterEach(() => {
    vi.unstubAllGlobals();
});

describe("createAuthClient", () => {
    it("selects the strategy from the mode", () => {
        expect(createAuthClient("cookie", BASE).mode).toBe("cookie");
        expect(createAuthClient("bearer", BASE).mode).toBe("bearer");
    });
});

describe("cookie auth", () => {
    const client = createAuthClient("cookie", BASE);

    it("decorates fetch with credentials and preserves the init", () => {
        const init = client.decorate({ method: "POST" });
        expect(init.credentials).toBe("include");
        expect(init.method).toBe("POST");
    });

    it("sets withCredentials on an xhr", () => {
        const xhr = { withCredentials: false } as XMLHttpRequest;
        client.applyXhr(xhr);
        expect(xhr.withCredentials).toBe(true);
    });

    it("logs out against the auth path without hitting the network", async () => {
        const fetchMock = vi.fn().mockResolvedValue(new Response(null));
        vi.stubGlobal("fetch", fetchMock);
        await client.signOut();
        expect(fetchMock).toHaveBeenCalledTimes(1);
        const [url, init] = fetchMock.mock.calls[0];
        expect(url).toBe(`${BASE}/api/unstable/auth/logout`);
        expect(init).toMatchObject({
            method: "POST",
            credentials: "include",
        });
    });
});

describe("bearer auth", () => {
    const client = createAuthClient("bearer", BASE);

    it("omits cookies and sends no Authorization header without a token", () => {
        const init = client.decorate();
        expect(init.credentials).toBe("omit");
        const headers = new Headers(init.headers);
        expect(headers.has("Authorization")).toBe(false);
    });

    it("resolves signOut without any network call", async () => {
        await expect(client.signOut()).resolves.toBeUndefined();
    });
});
