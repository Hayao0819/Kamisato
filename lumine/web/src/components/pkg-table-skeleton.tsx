"use client";

// Loading placeholder for the package list. Mirrors the real table's column
// set and row height so switching between loading and loaded states does not
// shift the layout horizontally.
const COL_WIDTHS = [
    "w-40",
    "w-24",
    "w-full",
    "w-16",
    "w-20",
    "w-16",
    "w-24",
    "w-16",
];

export function PkgTableSkeleton({ rows = 12 }: { rows?: number }) {
    return (
        <div className="space-y-3">
            <div className="flex flex-wrap items-center gap-2">
                <div className="h-9 w-44 animate-pulse rounded-sm bg-muted" />
                <div className="h-9 w-28 animate-pulse rounded-sm bg-muted" />
                <div className="h-9 w-28 animate-pulse rounded-sm bg-muted" />
            </div>
            <div className="h-5 w-48 animate-pulse rounded-sm bg-muted" />
            <div className="overflow-x-auto rounded-sm border border-border bg-card">
                <table className="arch-table w-full text-[15px]">
                    <thead>
                        <tr>
                            {COL_WIDTHS.map((_, i) => (
                                <th
                                    // biome-ignore lint/suspicious/noArrayIndexKey: fixed-length placeholder
                                    key={i}
                                    className="px-4 py-2.5 text-left"
                                >
                                    <div className="h-4 w-16 animate-pulse rounded-sm bg-muted" />
                                </th>
                            ))}
                        </tr>
                    </thead>
                    <tbody>
                        {Array.from({ length: rows }).map((_, r) => (
                            <tr
                                // biome-ignore lint/suspicious/noArrayIndexKey: fixed-length placeholder
                                key={r}
                            >
                                {COL_WIDTHS.map((w, c) => (
                                    <td
                                        // biome-ignore lint/suspicious/noArrayIndexKey: fixed-length placeholder
                                        key={c}
                                        className="px-4 py-3"
                                    >
                                        <div
                                            className={`h-4 ${w} max-w-full animate-pulse rounded-sm bg-muted`}
                                        />
                                    </td>
                                ))}
                            </tr>
                        ))}
                    </tbody>
                </table>
            </div>
        </div>
    );
}
