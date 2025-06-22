const isDev = process.env.NODE_ENV === "development";
/** @type {import('next').NextConfig} */
const nextConfig = {
    eslint: {
        ignoreDuringBuilds: true,
    },
    typescript: {
        ignoreBuildErrors: true,
    },
    images: {
        unoptimized: true,
    },
    distDir: isDev ? ".next" : "../embed/out",
    output: "export",
    trailingSlash: true,
};

export default nextConfig;
