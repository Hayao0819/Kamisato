import { readdirSync, readFileSync } from "node:fs";
import { describe, expect, it } from "vitest";

describe("client library dependencies", () => {
    const directory = new URL(".", import.meta.url);
    const files = readdirSync(directory).filter(
        (name) => name.endsWith(".ts") && !name.endsWith(".test.ts"),
    );

    it.each(
        files,
    )("%s does not depend on views, hooks, or React state", (name) => {
        const source = readFileSync(new URL(name, directory), "utf8");
        const imports = Array.from(
            source.matchAll(/(?:from\s+|import\s*)["']([^"']+)["']/g),
            (match) => match[1],
        );
        for (const dependency of imports) {
            expect(dependency, `${name} imports ${dependency}`).not.toMatch(
                /(?:^|\/)(?:components|hooks)(?:\/|$)|^(?:react(?:-dom)?|next|jotai)(?:\/|$)/,
            );
        }
    });
});
