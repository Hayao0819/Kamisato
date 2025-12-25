/** @type {import('next').NextConfig} */
const nextConfig = {
    typescript: {
        ignoreBuildErrors: true,
    },
    images: {
        unoptimized: true,
    },
    distDir: "out",
    output: "export",
    trailingSlash: true,
};

export default nextConfig;
