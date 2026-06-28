import { fileURLToPath } from "node:url";

/** @type {import('next').NextConfig} */
const isDev = process.env.NODE_ENV === "development";

const nextConfig = {
    typescript: {
        ignoreBuildErrors: true,
    },
    images: {
        unoptimized: true,
    },
    trailingSlash: true,
    turbopack: { root: fileURLToPath(new URL(".", import.meta.url)) },
    // Dev: proxy /api and /repo to ayato (same origin), mirroring the lumine binary.
    // Prod is a static export served by that binary, which does the proxying.
    ...(isDev
        ? {
              async rewrites() {
                  const ayato =
                      process.env.AYATO_URL || "http://localhost:8080";
                  return [
                      {
                          source: "/api/:path*",
                          destination: `${ayato}/api/:path*`,
                      },
                      {
                          source: "/repo/:path*",
                          destination: `${ayato}/repo/:path*`,
                      },
                  ];
              },
          }
        : { output: "export", distDir: "../embed/out" }),
};

export default nextConfig;
