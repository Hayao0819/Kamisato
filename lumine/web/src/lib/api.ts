export const API_BASE_URL =
    process.env.NEXT_PUBLIC_API_BASE_URL || "http://localhost:9000";

export const getHelloEndpoint = () => `${API_BASE_URL}/api/unstable/hello`;
export const getTeapotEndpoint = () => `${API_BASE_URL}/api/unstable/teapot`;
export const getAllPkgsEndpoint = (repo: string, arch: string) =>
    `${API_BASE_URL}/api/unstable/${repo}/${arch}/package`;
export const getRepoFileListEndpoint = (repo: string, arch: string) =>
    `${API_BASE_URL}/repo/${repo}/${arch}`;
export const getRepoFileEndpoint = (repo: string, arch: string, file: string) =>
    `${API_BASE_URL}/repo/${repo}/${arch}/${file}`;

export const getReposEndpoint = () => `${API_BASE_URL}/api/unstable/repos`;
export const getArchesEndpoint = (repo: string) =>
    `${API_BASE_URL}/api/unstable/repos/${repo}/archs`;
