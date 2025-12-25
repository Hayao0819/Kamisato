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
    // APIリクエストラッパー
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
}
