"use client";

import Link from "next/link";

// Strips a version constraint / descriptor from a dependency atom so the bare
// package name can be used as a /package pkgbase. Handles "name>=1.0",
// "name=1.0", "name<2", and optdepend's "name: description" form.
function depName(atom: string): string {
    const base = atom.split(":")[0].trim();
    const m = base.match(/^[^<>=!]+/);
    return (m ? m[0] : base).trim();
}

interface PkgDepSectionProps {
    title: string;
    items: string[] | null;
    repo?: string;
    arch?: string;
    // When set, each chip links to the package detail of the dependency within
    // the same repo/arch scope. Used for hard/build/check deps that resolve to
    // packages in this repo; left off for provides/conflicts/replaces/optdepend.
    link?: boolean;
}

export function PkgDepSection({
    title,
    items,
    repo,
    arch,
    link,
}: PkgDepSectionProps) {
    const canLink = Boolean(link && repo && arch);
    const list = items ?? [];

    return (
        <section className="space-y-2">
            <h2 className="text-[15px] font-semibold">
                {title}{" "}
                <span className="font-normal tabular-nums text-muted-foreground">
                    ({list.length})
                </span>
            </h2>
            {list.length === 0 ? (
                <p className="text-[14px] text-muted-foreground/60">なし</p>
            ) : (
                <ul className="flex flex-wrap gap-1.5">
                    {list.map((item) => (
                        <li key={item}>
                            {canLink ? (
                                <Link
                                    href={`/package?repo=${encodeURIComponent(repo as string)}&arch=${encodeURIComponent(arch as string)}&pkgbase=${encodeURIComponent(depName(item))}`}
                                    className="inline-flex items-center rounded-sm border border-border bg-muted px-2 py-0.5 font-mono text-[13px] text-link hover:border-primary hover:underline focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring"
                                >
                                    {item}
                                </Link>
                            ) : (
                                <span className="inline-flex items-center rounded-sm border border-border bg-muted px-2 py-0.5 font-mono text-[13px]">
                                    {item}
                                </span>
                            )}
                        </li>
                    ))}
                </ul>
            )}
        </section>
    );
}
