import type { JobStatus } from "@/lib/types";

export const STATUS_VARIANT: Record<
    JobStatus,
    "default" | "secondary" | "destructive" | "outline"
> = {
    queued: "secondary",
    running: "default",
    success: "outline",
    failed: "destructive",
    cancelled: "outline",
};

export const STATUS_LABEL: Record<JobStatus, string> = {
    queued: "待機中",
    running: "実行中",
    success: "成功",
    failed: "失敗",
    cancelled: "取消",
};

export const STATUS_ORDER: JobStatus[] = [
    "queued",
    "running",
    "success",
    "failed",
    "cancelled",
];

export function isActive(status: JobStatus): boolean {
    return status === "queued" || status === "running";
}

export function formatDuration(startedAt?: string, endedAt?: string): string {
    if (!startedAt) return "—";
    const start = new Date(startedAt).getTime();
    const end = endedAt ? new Date(endedAt).getTime() : Date.now();
    if (!Number.isFinite(start) || !Number.isFinite(end)) return "—";
    const sec = Math.max(0, Math.round((end - start) / 1000));
    if (sec < 60) return `${sec}秒`;
    const min = Math.floor(sec / 60);
    const rem = sec % 60;
    if (min < 60) return rem ? `${min}分${rem}秒` : `${min}分`;
    const hour = Math.floor(min / 60);
    const minRem = min % 60;
    return minRem ? `${hour}時間${minRem}分` : `${hour}時間`;
}
