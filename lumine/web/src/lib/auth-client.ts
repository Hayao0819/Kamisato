// Auth delivery strategies. The SPA talks to ayato either same-origin with an
// HttpOnly session cookie (cookie mode, behind the lumine BFF/edge proxy) or
// cross-origin with a bearer token it holds in memory (bearer mode, a fully
// static SPA on a different origin). The mode is chosen at runtime from
// /env.json's AUTH_MODE, so the same bundle serves both deployments.

export type AuthMode = "cookie" | "bearer";

export interface AuthClient {
    readonly mode: AuthMode;
    // decorate applies the credential to a fetch init for this mode.
    decorate(init?: RequestInit): RequestInit;
    // applyXhr applies the credential to an XHR; call it after xhr.open().
    applyXhr(xhr: XMLHttpRequest): void;
    // signIn starts a login. Cookie mode navigates the top-level window and never
    // resolves (the page unloads); bearer mode opens a popup, completes PKCE, and
    // resolves once the in-memory token is set.
    signIn(): Promise<void>;
    // signOut ends the session. Cookie mode clears the server cookie; bearer mode
    // drops the in-memory token.
    signOut(): Promise<void>;
}

export function createAuthClient(mode: AuthMode, base: string): AuthClient {
    return mode === "bearer"
        ? new BearerAuthClient(base)
        : new CookieAuthClient(base);
}

const authPath = (base: string, p: string) => `${base}/api/unstable/auth/${p}`;

class CookieAuthClient implements AuthClient {
    readonly mode: AuthMode = "cookie";
    constructor(private readonly base: string) {}

    decorate(init: RequestInit = {}): RequestInit {
        return { ...init, credentials: "include" };
    }

    applyXhr(xhr: XMLHttpRequest): void {
        xhr.withCredentials = true;
    }

    signIn(): Promise<void> {
        // The 302-to-GitHub redirect and the Lax cookie only work on a top-level
        // navigation, never a fetch. The page unloads, so this never resolves.
        window.location.assign(authPath(this.base, "github/login"));
        return new Promise<void>(() => {});
    }

    async signOut(): Promise<void> {
        try {
            await fetch(authPath(this.base, "logout"), {
                method: "POST",
                credentials: "include",
            });
        } catch {
            // best-effort: the cookie expires by TTL regardless
        }
    }
}

class BearerAuthClient implements AuthClient {
    readonly mode: AuthMode = "bearer";
    private token: string | null = null;
    constructor(private readonly base: string) {}

    decorate(init: RequestInit = {}): RequestInit {
        const headers = new Headers(init.headers);
        if (this.token) headers.set("Authorization", `Bearer ${this.token}`);
        // No cookies cross-origin: the bearer token is the sole credential.
        return { ...init, headers, credentials: "omit" };
    }

    applyXhr(xhr: XMLHttpRequest): void {
        if (this.token)
            xhr.setRequestHeader("Authorization", `Bearer ${this.token}`);
    }

    async signIn(): Promise<void> {
        const verifier = randomString(32);
        const challenge = await s256(verifier);
        const state = randomString(16);
        const ayatoOrigin = new URL(this.base).origin;

        const startUrl = `${authPath(this.base, "web/start")}?challenge=${encodeURIComponent(challenge)}&state=${encodeURIComponent(state)}`;
        const popup = window.open(
            startUrl,
            "ayato-login",
            "width=520,height=680",
        );
        if (!popup)
            throw new Error(
                "ログインポップアップを開けませんでした。ポップアップを許可してください",
            );

        const code = await awaitCode(ayatoOrigin, state, popup);
        const res = await fetch(authPath(this.base, "web/exchange"), {
            method: "POST",
            headers: { "Content-Type": "application/json" },
            body: JSON.stringify({ code, code_verifier: verifier }),
        });
        if (!res.ok) throw new Error(`ログインに失敗しました: ${res.status}`);
        const data = (await res.json()) as { token: string };
        this.token = data.token;
    }

    async signOut(): Promise<void> {
        // The token lives only in memory; dropping it ends the session. There is
        // no server cookie to clear, and the signed token expires by its TTL.
        this.token = null;
    }
}

// awaitCode resolves with the one-time code the popup posts back. It accepts only
// a message from the ayato origin whose state matches the one this flow sent, so
// a foreign frame cannot inject a code.
function awaitCode(
    ayatoOrigin: string,
    expectedState: string,
    popup: Window,
): Promise<string> {
    return new Promise<string>((resolve, reject) => {
        const onMessage = (e: MessageEvent) => {
            if (e.origin !== ayatoOrigin) return;
            const d = e.data as
                | { type?: string; code?: string; state?: string }
                | undefined;
            if (!d || d.type !== "ayato-auth" || d.state !== expectedState)
                return;
            if (typeof d.code !== "string") return;
            cleanup();
            resolve(d.code);
        };
        const timer = window.setTimeout(() => {
            cleanup();
            reject(new Error("ログインがタイムアウトしました"));
        }, 120_000);
        const poll = window.setInterval(() => {
            if (popup.closed) {
                cleanup();
                reject(new Error("ログインがキャンセルされました"));
            }
        }, 500);
        function cleanup() {
            window.clearTimeout(timer);
            window.clearInterval(poll);
            window.removeEventListener("message", onMessage);
        }
        window.addEventListener("message", onMessage);
    });
}

function randomString(bytes: number): string {
    const a = new Uint8Array(bytes);
    crypto.getRandomValues(a);
    return base64url(a);
}

async function s256(verifier: string): Promise<string> {
    const digest = await crypto.subtle.digest(
        "SHA-256",
        new TextEncoder().encode(verifier),
    );
    return base64url(new Uint8Array(digest));
}

function base64url(bytes: Uint8Array): string {
    let s = "";
    for (const b of bytes) s += String.fromCharCode(b);
    return btoa(s).replace(/\+/g, "-").replace(/\//g, "_").replace(/=+$/, "");
}
