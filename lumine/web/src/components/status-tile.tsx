"use client";

import type { ReactNode } from "react";
import { cn } from "@/lib/utils";

export function StatusTile({
    label,
    value,
    hint,
    icon,
    className,
}: {
    label: string;
    value: ReactNode;
    hint?: ReactNode;
    icon?: ReactNode;
    className?: string;
}) {
    return (
        <div
            className={cn(
                "rounded-md border bg-card p-4 text-card-foreground",
                className,
            )}
        >
            <div className="flex items-center justify-between gap-2">
                <span className="text-sm text-muted-foreground">{label}</span>
                {icon && <span className="text-muted-foreground">{icon}</span>}
            </div>
            <div className="mt-2 text-2xl font-semibold tabular-nums tracking-tight">
                {value}
            </div>
            {hint && (
                <div className="mt-1 text-xs text-muted-foreground">{hint}</div>
            )}
        </div>
    );
}
