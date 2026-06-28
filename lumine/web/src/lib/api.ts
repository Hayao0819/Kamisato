import {
    type AuthClient,
    type AuthMode,
    createAuthClient,
} from "./auth-client";
import type { BuildRequest, BuildStats, Job } from "./types";

type LumineEnv = {
    AYATO_URL: string | null;
    AUTH_MODE?: AuthMode;
    FALLBACK: boolean;
};

const fallbackLumineEnv: LumineEnv = {
    AYATO_URL: null,
    AUTH_MODE: "cookie",
    FALLBACK: true,
};

// Carries the HTTP status so the UI can tell "not signed in" (401) from
// "signed in but not authorized / CSRF" (403). The message is what toast
// handlers surface to the user.
export class MutationError extends Error {
    readonly status: number;
    constructor(status: number, message: string) {
        super(message);
        this.name = "MutationError";
        this.status = status;
    }
}

function mutationError(
    status: number,
    fallback: string,
    detail?: string,
): MutationError {
    if (status === 401) {
        return new MutationError(status, "ログインが必要です");
    }
    if (status === 403) {
        return new MutationError(status, "権限がありません");
    }
    const suffix = detail ? ` - ${detail}` : "";
    return new MutationError(status, `${fallback}: ${status}${suffix}`);
}

export class APIClient {
    async fetchAllPkgs(repo: string, arch: string) {
        const res = await this.authedFetch(this.endpoints.allPkgs(repo, arch));
        if (!res.ok)
            throw new Error(`Failed to fetch packages: ${res.statusText}`);
        return res.json();
    }

    async fetchPackageDetail(repo: string, arch: string, pkgbase: string) {
        const res = await this.authedFetch(
            this.endpoints.packageDetail(repo, arch, pkgbase),
        );
        if (!res.ok) throw new Error("パッケージ情報の取得に失敗しました");
        return res.json();
    }

    async fetchRepos() {
        const res = await this.authedFetch(this.endpoints.repos());
        if (!res.ok)
            throw new Error(
                `リポジトリ一覧の取得に失敗しました: ${res.status}`,
            );
        return res.json();
    }

    async fetchArches(repo: string) {
        const res = await this.authedFetch(this.endpoints.arches(repo));
        if (!res.ok)
            throw new Error(
                `アーキテクチャ一覧の取得に失敗しました: ${res.status}`,
            );
        return res.json();
    }

    async fetchHello() {
        return this.authedFetch(this.endpoints.hello());
    }

    async fetchTeapot() {
        return this.authedFetch(this.endpoints.teapot());
    }

    // Probes the active session: cookie mode sends the HttpOnly cookie
    // first-party, bearer mode attaches the Authorization header (both through
    // authedFetch). Any failure is treated as "not signed in".
    async fetchMe(): Promise<{
        authenticated: boolean;
        id?: number;
        login?: string;
    }> {
        try {
            const res = await this.authedFetch(this.endpoints.authMe());
            if (!res.ok) return { authenticated: false };
            return res.json();
        } catch {
            return { authenticated: false };
        }
    }

    async uploadPackage(
        repo: string,
        packageFile: File,
        signatureFile: File | null,
    ) {
        const formData = new FormData();
        formData.append("package", packageFile);
        if (signatureFile) {
            formData.append("signature", signatureFile);
        }

        const res = await this.authedFetch(this.endpoints.uploadPackage(repo), {
            method: "PUT",
            body: formData,
        });

        if (!res.ok) {
            const errorText = await res.text();
            throw mutationError(
                res.status,
                `パッケージのアップロードに失敗しました`,
                errorText,
            );
        }

        return res.text();
    }

    // Build jobs are owned by miko; ayato proxies these endpoints, so clients
    // never address miko directly.
    async submitBuild(req: BuildRequest): Promise<{ job_id: string }> {
        const res = await this.authedFetch(this.endpoints.submitBuild(), {
            method: "POST",
            headers: { "Content-Type": "application/json" },
            body: JSON.stringify(req),
        });

        if (!res.ok) {
            const errorText = await res.text();
            throw mutationError(
                res.status,
                `ビルドの投入に失敗しました`,
                errorText,
            );
        }

        return res.json();
    }

    async cancelJob(id: string): Promise<void> {
        const res = await this.authedFetch(this.endpoints.cancelJob(id), {
            method: "DELETE",
        });

        if (!res.ok) {
            const errorText = await res.text();
            throw mutationError(
                res.status,
                `ジョブのキャンセルに失敗しました`,
                errorText,
            );
        }
    }

    async fetchStats(): Promise<BuildStats> {
        const res = await this.authedFetch(this.endpoints.stats());
        if (!res.ok)
            throw new Error(
                `ビルドサーバーの状態取得に失敗しました: ${res.status}`,
            );
        return res.json();
    }

    async listJobs(): Promise<Job[]> {
        const res = await this.authedFetch(this.endpoints.listJobs());
        if (!res.ok)
            throw new Error(`ジョブ一覧の取得に失敗しました: ${res.status}`);
        return res.json();
    }

    async jobDetail(id: string): Promise<Job> {
        const res = await this.authedFetch(this.endpoints.jobDetail(id));
        if (!res.ok)
            throw new Error(`ジョブ情報の取得に失敗しました: ${res.status}`);
        return res.json();
    }

    jobLogsUrl(id: string): string {
        return this.endpoints.jobLogs(id);
    }

    uploadPackageWithProgress(
        repo: string,
        packageFile: File,
        signatureFile: File | null,
        onProgress?: (progress: number) => void,
    ): Promise<string> {
        return new Promise((resolve, reject) => {
            const formData = new FormData();
            formData.append("package", packageFile);
            if (signatureFile) {
                formData.append("signature", signatureFile);
            }

            const xhr = new XMLHttpRequest();

            xhr.upload.addEventListener("progress", (e) => {
                if (e.lengthComputable && onProgress) {
                    const progress = (e.loaded / e.total) * 100;
                    onProgress(progress);
                }
            });

            xhr.addEventListener("load", () => {
                if (xhr.status >= 200 && xhr.status < 300) {
                    resolve(xhr.responseText);
                } else {
                    reject(
                        mutationError(
                            xhr.status,
                            "パッケージのアップロードに失敗しました",
                            xhr.responseText,
                        ),
                    );
                }
            });

            xhr.addEventListener("error", () => {
                reject(new Error("ネットワークエラーが発生しました"));
            });

            xhr.addEventListener("abort", () => {
                reject(new Error("アップロードがキャンセルされました"));
            });

            xhr.open("PUT", this.endpoints.uploadPackage(repo));
            this.auth.applyXhr(xhr);
            xhr.send(formData);
        });
    }

    readonly lumineEnv: LumineEnv;
    readonly endpoints: APIEndpoints;
    readonly auth: AuthClient;

    constructor(lumineEnv?: LumineEnv) {
        this.lumineEnv = lumineEnv || fallbackLumineEnv;
        this.endpoints = new APIEndpoints(this.serverUrl);
        this.auth = createAuthClient(
            this.lumineEnv.AUTH_MODE ?? "cookie",
            this.serverUrl ?? "",
        );
    }

    // authedFetch applies the active auth strategy: cookie mode sends the session
    // cookie (credentials: include); bearer mode attaches the Authorization
    // header and omits cookies. Every authenticated call routes through here.
    private authedFetch(input: string, init?: RequestInit): Promise<Response> {
        return fetch(input, this.auth.decorate(init));
    }

    signIn(): Promise<void> {
        return this.auth.signIn();
    }

    signOut(): Promise<void> {
        return this.auth.signOut();
    }

    static async init(): Promise<APIClient> {
        let lumineEnv: LumineEnv | undefined;
        try {
            const res = await fetch("/env.json", { cache: "no-store" });
            const json = await res.json();
            lumineEnv = json;
            return new APIClient(lumineEnv);
        } catch (e) {
            console.warn("Failed to fetch lumineEnv, using defaults", e);
            lumineEnv = fallbackLumineEnv;
        }
        return new APIClient(lumineEnv);
    }

    static fallback(): APIClient {
        return new APIClient(fallbackLumineEnv);
    }

    get serverUrl(): string | null {
        return this.lumineEnv.AYATO_URL;
    }
}

class APIEndpoints {
    private readonly base: string | null;
    constructor(base: string | null) {
        this.base = base;
    }
    get executable(): boolean {
        return this.base !== null;
    }
    get apiUnstableUrl(): string {
        return `${this.base}/api/unstable`;
    }
    get packageDetail() {
        return (repo: string, arch: string, pkgbase: string) =>
            `${this.apiUnstableUrl}/${repo}/${arch}/package/${pkgbase}`;
    }
    get hello() {
        return () => `${this.apiUnstableUrl}/hello`;
    }
    get teapot() {
        return () => `${this.apiUnstableUrl}/teapot`;
    }
    get authMe() {
        return () => `${this.apiUnstableUrl}/auth/me`;
    }
    get logout() {
        return () => `${this.apiUnstableUrl}/auth/logout`;
    }
    get githubLogin() {
        return () => `${this.apiUnstableUrl}/auth/github/login`;
    }
    get allPkgs() {
        return (repo: string, arch: string) =>
            `${this.apiUnstableUrl}/${repo}/${arch}/package`;
    }
    get repoFileList() {
        return (repo: string, arch: string) =>
            `${this.base}/repo/${repo}/${arch}`;
    }
    get repoFile() {
        return (repo: string, arch: string, file: string) =>
            `${this.base}/repo/${repo}/${arch}/${file}`;
    }
    get repos() {
        return () => `${this.apiUnstableUrl}/repos`;
    }
    get arches() {
        return (repo: string) => `${this.apiUnstableUrl}/repos/${repo}/archs`;
    }
    get uploadPackage() {
        return (repo: string) => `${this.apiUnstableUrl}/${repo}/package`;
    }
    get submitBuild() {
        return () => `${this.apiUnstableUrl}/build`;
    }
    get listJobs() {
        return () => `${this.apiUnstableUrl}/jobs`;
    }
    get jobDetail() {
        return (id: string) => `${this.apiUnstableUrl}/jobs/${id}`;
    }
    get jobLogs() {
        return (id: string) => `${this.apiUnstableUrl}/jobs/${id}/logs`;
    }
    get cancelJob() {
        return (id: string) => `${this.apiUnstableUrl}/jobs/${id}`;
    }
    get stats() {
        return () => `${this.apiUnstableUrl}/stats`;
    }
}
