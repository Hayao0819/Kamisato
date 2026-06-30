import { fileURLToPath } from "node:url";
import { defineConfig } from "vitest/config";

const srcDir = fileURLToPath(new URL("./src", import.meta.url));

export default defineConfig({
    test: {
        environment: "node",
        include: ["src/**/*.test.ts"],
    },
    resolve: {
        alias: [{ find: /^@\//, replacement: `${srcDir}/` }],
    },
});
