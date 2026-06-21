"use client";

import Link from "next/link";
import { Activity, Clock, Loader2 } from "lucide-react";
import type { BuildStats } from "@/lib/types";

export function BuildQueueIndicator({ stats }: { stats: BuildStats | null }) {
    const running = stats?.running ?? 0;
    const queued = stats?.queue_length ?? 0;
    const workers = stats?.workers ?? 0;
    const live = running > 0;

    return (
        <div className="flex flex-wrap items-center gap-x-4 gap-y-1.5 rounded-md border border-border bg-card px-3 py-2 text-[13px]">
            <span className="inline-flex items-center gap-1.5">
                {live ? (
                    <Loader2 className="h-3.5 w-3.5 animate-spin text-primary" />
                ) : (
                    <Activity className="h-3.5 w-3.5 text-muted-foreground" />
                )}
                <span className="text-muted-foreground">実行中</span>
                <span className="tabular-nums font-medium text-foreground">
                    {running}
                </span>
            </span>
            <span className="inline-flex items-center gap-1.5">
                <Clock className="h-3.5 w-3.5 text-muted-foreground" />
                <span className="text-muted-foreground">待機</span>
                <span className="tabular-nums font-medium text-foreground">
                    {queued}
                </span>
            </span>
            <span className="text-muted-foreground/70">
                ワーカー{" "}
                <span className="tabular-nums text-foreground">{workers}</span>
            </span>
            <Link
                href="/server-status"
                className="ml-auto text-[12px] text-(--link) hover:underline"
            >
                サーバー状態
            </Link>
        </div>
    );
}
