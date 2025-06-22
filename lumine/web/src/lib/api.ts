declare global {
    interface Window {
        lumineEnv?: {
            AYATO_URL?: string;
            SERVER_CONFIGURABLE?: boolean;
        };
    }
}

export async function fetchLumineEnv() {
    if (typeof window === "undefined") return;
    if (window.lumineEnv) return;
    try {
        const res = await fetch("/env.json", { cache: "no-store" });
        const json = await res.json();
        window.lumineEnv = json;
    } catch (e) {
        // 失敗時はデフォルト
        window.lumineEnv = {
            AYATO_URL: "http://localhost:9000",
            SERVER_CONFIGURABLE: true,
        };
    }
}

function getLumineEnvSync() {
    if (typeof window !== "undefined" && window.lumineEnv) {
        return window.lumineEnv;
    }
    // SSRや未ロード時はデフォルト値
    return {
        AYATO_URL: "http://localhost:9000",
        SERVER_CONFIGURABLE: true,
    };
}

export const SERVER_CONFIGURABLE =
    typeof window !== "undefined"
        ? (window.lumineEnv?.SERVER_CONFIGURABLE ?? true)
        : true;

function getApiBaseUrl(): string {
    const env = getLumineEnvSync();
    if (env.SERVER_CONFIGURABLE && typeof window !== "undefined") {
        const localUrl = window.localStorage.getItem("lumine_api_base_url");
        if (localUrl && localUrl.trim()) return localUrl.trim();
    }
    return env.AYATO_URL || "http://localhost:9000";
}

// パッケージ詳細取得用エンドポイント
export const getPackageDetailEndpoint = (
    repo: string,
    arch: string,
    pkgbase: string,
) => `${getApiBaseUrl()}/api/unstable/${repo}/${arch}/package/${pkgbase}`;

export const API_BASE_URL = getApiBaseUrl();

export const getHelloEndpoint = () => `${getApiBaseUrl()}/api/unstable/hello`;
export const getTeapotEndpoint = () => `${getApiBaseUrl()}/api/unstable/teapot`;
export const getAllPkgsEndpoint = (repo: string, arch: string) =>
    `${getApiBaseUrl()}/api/unstable/${repo}/${arch}/package`;
export const getRepoFileListEndpoint = (repo: string, arch: string) =>
    `${getApiBaseUrl()}/repo/${repo}/${arch}`;
export const getRepoFileEndpoint = (repo: string, arch: string, file: string) =>
    `${getApiBaseUrl()}/repo/${repo}/${arch}/${file}`;

export const getReposEndpoint = () => `${getApiBaseUrl()}/api/unstable/repos`;
export const getArchesEndpoint = (repo: string) =>
    `${getApiBaseUrl()}/api/unstable/repos/${repo}/archs`;
