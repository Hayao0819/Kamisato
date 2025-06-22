// パッケージ詳細取得用エンドポイント
export const getPackageDetailEndpoint = (repo: string, arch: string, pkgbase: string) =>
    `${getApiBaseUrl()}/api/unstable/${repo}/${arch}/package/${pkgbase}`;
function getApiBaseUrl(): string {
    if (typeof window !== "undefined") {
        return (
            window.localStorage.getItem("lumine_api_base_url") ||
            process.env.NEXT_PUBLIC_API_BASE_URL ||
            "http://localhost:9000"
        );
    }
    return process.env.NEXT_PUBLIC_API_BASE_URL || "http://localhost:9000";
}

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
