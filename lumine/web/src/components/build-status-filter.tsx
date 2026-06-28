"use client";

import type { JobStatus } from "@/lib/types";
import { cn } from "@/lib/utils";
import { STATUS_LABEL, STATUS_ORDER } from "./build-status";

export type StatusFilter = JobStatus | "all";

export function BuildStatusFilter({
    value,
    counts,
    total,
    onChange,
}: {
    value: StatusFilter;
    counts: Record<JobStatus, number>;
    total: number;
    onChange: (next: StatusFilter) => void;
}) {
    const chips: { key: StatusFilter; label: string; count: number }[] = [
        { key: "all", label: "すべて", count: total },
        ...STATUS_ORDER.map((s) => ({
            key: s as StatusFilter,
            label: STATUS_LABEL[s],
            count: counts[s] ?? 0,
        })),
    ];

    return (
        <div className="flex flex-wrap items-center gap-1.5">
            {chips.map((chip) => {
                const active = value === chip.key;
                return (
                    <button
                        key={chip.key}
                        type="button"
                        onClick={() => onChange(chip.key)}
                        aria-pressed={active}
                        className={cn(
                            "inline-flex items-center gap-1.5 rounded-sm border px-2.5 py-1 text-[13px] transition-colors",
                            active
                                ? "border-transparent bg-primary text-primary-foreground"
                                : "border-border bg-card text-muted-foreground hover:bg-muted hover:text-foreground",
                        )}
                    >
                        <span>{chip.label}</span>
                        <span
                            className={cn(
                                "tabular-nums text-[12px]",
                                active
                                    ? "text-primary-foreground/80"
                                    : "text-muted-foreground/70",
                            )}
                        >
                            {chip.count}
                        </span>
                    </button>
                );
            })}
        </div>
    );
}
