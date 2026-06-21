import type { BuildRequest, BuildStats, Job } from "./types";

type LumineEnv = {
    AYATO_URL: string | null;
    SERVER_CONFIGURABLE: boolean;
    FALLBACK: boolean;
};

const fallbackLumineEnv: LumineEnv = {
    AYATO_URL: null,
    SERVER_CONFIGURABLE: true,
    FALLBACK: true,
};

export class APIClient {
    async fetchAllPkgs(repo: string, arch: string) {
        const res = await fetch(this.endpoints.allPkgs(repo, arch));
        if (!res.ok)
            throw new Error(`Failed to fetch packages: ${res.statusText}`);
        return res.json();
    }

    async fetchPackageDetail(repo: string, arch: string, pkgbase: string) {
        const res = await fetch(
            this.endpoints.packageDetail(repo, arch, pkgbase),
        );
        if (!res.ok) throw new Error("パッケージ情報の取得に失敗しました");
        return res.json();
    }

    async fetchRepos() {
        const res = await fetch(this.endpoints.repos());
        if (!res.ok)
            throw new Error(
                `リポジトリ一覧の取得に失敗しました: ${res.status}`,
            );
        return res.json();
    }

    async fetchArches(repo: string) {
        const res = await fetch(this.endpoints.arches(repo));
        if (!res.ok)
            throw new Error(
                `アーキテクチャ一覧の取得に失敗しました: ${res.status}`,
            );
        return res.json();
    }

    async fetchHello() {
        return fetch(this.endpoints.hello());
    }

    async fetchTeapot() {
        return fetch(this.endpoints.teapot());
    }

    async fetchAuthRequired() {
        try {
            const res = await fetch(this.endpoints.authRequired());
            if (!res.ok) return { required: false };
            return res.json();
        } catch {
            return { required: false };
        }
    }

    async uploadPackage(
        repo: string,
        packageFile: File,
        signatureFile: File | null,
        username?: string,
        password?: string,
    ) {
        const formData = new FormData();
        formData.append("package", packageFile);
        if (signatureFile) {
            formData.append("signature", signatureFile);
        }

        const headers: HeadersInit = {};
        if (username && password) {
            const credentials = btoa(`${username}:${password}`);
            headers.Authorization = `Basic ${credentials}`;
        }

        const res = await fetch(this.endpoints.uploadPackage(repo), {
            method: "PUT",
            headers,
            body: formData,
        });

        if (!res.ok) {
            const errorText = await res.text();
            throw new Error(
                `パッケージのアップロードに失敗しました: ${res.status} - ${errorText}`,
            );
        }

        return res.text();
    }

    // Build jobs are owned by miko; ayato proxies these endpoints, so clients
    // never address miko directly.
    async submitBuild(
        req: BuildRequest,
        username?: string,
        password?: string,
    ): Promise<{ job_id: string }> {
        const headers: HeadersInit = { "Content-Type": "application/json" };
        if (username && password) {
            const credentials = btoa(`${username}:${password}`);
            headers.Authorization = `Basic ${credentials}`;
        }

        const res = await fetch(this.endpoints.submitBuild(), {
            method: "POST",
            headers,
            body: JSON.stringify(req),
        });

        if (!res.ok) {
            const errorText = await res.text();
            throw new Error(
                `ビルドの投入に失敗しました: ${res.status} - ${errorText}`,
            );
        }

        return res.json();
    }

    async cancelJob(
        id: string,
        username?: string,
        password?: string,
    ): Promise<void> {
        const headers: HeadersInit = {};
        if (username && password) {
            const credentials = btoa(`${username}:${password}`);
            headers.Authorization = `Basic ${credentials}`;
        }

        const res = await fetch(this.endpoints.cancelJob(id), {
            method: "DELETE",
            headers,
        });

        if (!res.ok) {
            const errorText = await res.text();
            throw new Error(
                `ジョブのキャンセルに失敗しました: ${res.status} - ${errorText}`,
            );
        }
    }

    async fetchStats(): Promise<BuildStats> {
        const res = await fetch(this.endpoints.stats());
        if (!res.ok)
            throw new Error(
                `ビルドサーバーの状態取得に失敗しました: ${res.status}`,
            );
        return res.json();
    }

    async listJobs(): Promise<Job[]> {
        const res = await fetch(this.endpoints.listJobs());
        if (!res.ok)
            throw new Error(`ジョブ一覧の取得に失敗しました: ${res.status}`);
        return res.json();
    }

    async jobDetail(id: string): Promise<Job> {
        const res = await fetch(this.endpoints.jobDetail(id));
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
        username?: string,
        password?: string,
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
                        new Error(
                            `パッケージのアップロードに失敗しました: ${xhr.status} - ${xhr.responseText}`,
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

            if (username && password) {
                const credentials = btoa(`${username}:${password}`);
                xhr.setRequestHeader("Authorization", `Basic ${credentials}`);
            }

            xhr.send(formData);
        });
    }

    readonly lumineEnv: LumineEnv;
    readonly endpoints: APIEndpoints;

    constructor(lumineEnv?: LumineEnv) {
        this.lumineEnv = lumineEnv || fallbackLumineEnv;
        this.endpoints = new APIEndpoints(this.serverUrl);
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
        const env = this.lumineEnv;
        if (env.SERVER_CONFIGURABLE && typeof window !== "undefined") {
            const localUrl = window.localStorage.getItem("lumine_api_base_url");
            if (localUrl?.trim()) return localUrl.trim();
        }
        return env.AYATO_URL;
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
    get authRequired() {
        return () => `${this.apiUnstableUrl}/auth/required`;
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
