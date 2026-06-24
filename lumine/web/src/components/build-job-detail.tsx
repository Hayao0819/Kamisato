"use client";

import { Ban, Check, Copy, Loader2, ScrollText } from "lucide-react";
import { useEffect, useRef, useState } from "react";
import { AuthGate } from "@/components/auth-gate";
import { useAPIClient } from "@/components/lumine-provider";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import {
    Dialog,
    DialogContent,
    DialogHeader,
    DialogTitle,
} from "@/components/ui/dialog";
import type { Job } from "@/lib/types";
import {
    STATUS_LABEL,
    STATUS_VARIANT,
    formatDuration,
    isActive,
} from "./build-status";

function MetaRow({
    label,
    children,
}: {
    label: string;
    children: React.ReactNode;
}) {
    return (
        <div className="flex flex-col gap-0.5">
            <dt className="text-[12px] text-muted-foreground">{label}</dt>
            <dd className="text-[14px] text-foreground">{children}</dd>
        </div>
    );
}

function pkgLabel(job: Job): string {
    const pkgs = job.packages ?? [];
    if (pkgs.length === 0) return "ビルドジョブ";
    const name =
        pkgs[0].split("/").pop()?.replace(/\.pkg\.tar\.[a-z0-9]+$/i, "") ??
        pkgs[0];
    return pkgs.length > 1 ? `${name} 他${pkgs.length - 1}件` : name;
}

export function BuildJobDetail({
    job,
    open,
    onOpenChange,
    onCancel,
}: {
    job: Job | null;
    open: boolean;
    onOpenChange: (open: boolean) => void;
    onCancel: (id: string) => Promise<void>;
}) {
    const api = useAPIClient();
    const [logLines, setLogLines] = useState<string[]>([]);
    const [cancelling, setCancelling] = useState(false);
    const [copied, setCopied] = useState(false);
    const logRef = useRef<HTMLPreElement>(null);
    const jobId = job?.id ?? null;

    useEffect(() => {
        if (!open || !jobId) return;
        setLogLines([]);
        const source = new EventSource(api.jobLogsUrl(jobId));
        source.onmessage = (e) => {
            setLogLines((prev) => [...prev, e.data]);
        };
        source.onerror = () => {
            source.close();
        };
        return () => source.close();
    }, [open, jobId, api]);

    // Pin the viewport to the tail as fresh lines stream in.
    useEffect(() => {
        const el = logRef.current;
        if (el) el.scrollTop = el.scrollHeight;
    }, [logLines]);

    const handleCancel = async () => {
        if (!jobId) return;
        setCancelling(true);
        try {
            await onCancel(jobId);
        } finally {
            setCancelling(false);
        }
    };

    const copyId = () => {
        if (!jobId) return;
        navigator.clipboard?.writeText(jobId);
        setCopied(true);
        setTimeout(() => setCopied(false), 1500);
    };

    return (
        <Dialog open={open} onOpenChange={onOpenChange}>
            <DialogContent className="max-h-[88vh] gap-5 overflow-y-auto sm:max-w-3xl">
                <DialogHeader>
                    <DialogTitle className="flex items-center gap-2">
                        <span className="font-mono text-[15px] break-all">
                            {job ? pkgLabel(job) : ""}
                        </span>
                        {job && (
                            <Badge variant={STATUS_VARIANT[job.status]}>
                                {STATUS_LABEL[job.status] ?? job.status}
                            </Badge>
                        )}
                    </DialogTitle>
                </DialogHeader>

                {job && (
                    <>
                        <div className="flex items-center gap-1.5 text-[12px] text-muted-foreground">
                            <span className="shrink-0">ジョブID</span>
                            <code className="break-all font-mono text-foreground/80">
                                {job.id}
                            </code>
                            <Button
                                type="button"
                                variant="ghost"
                                size="icon"
                                className="h-6 w-6 shrink-0"
                                aria-label="IDをコピー"
                                onClick={copyId}
                            >
                                {copied ? (
                                    <Check className="h-3.5 w-3.5" />
                                ) : (
                                    <Copy className="h-3.5 w-3.5" />
                                )}
                            </Button>
                        </div>
                        <dl className="grid grid-cols-2 gap-x-6 gap-y-3 sm:grid-cols-3">
                            <MetaRow label="リポジトリ">{job.repo}</MetaRow>
                            <MetaRow label="アーキテクチャ">
                                {job.arch}
                            </MetaRow>
                            <MetaRow label="作成日時">
                                {job.created_at
                                    ? new Date(job.created_at).toLocaleString(
                                          "ja-JP",
                                      )
                                    : "—"}
                            </MetaRow>
                            <MetaRow label="所要時間">
                                {formatDuration(job.started_at, job.ended_at)}
                            </MetaRow>
                            {typeof job.retries === "number" && (
                                <MetaRow label="リトライ">
                                    <span className="tabular-nums">
                                        {job.retries}
                                    </span>
                                </MetaRow>
                            )}
                        </dl>

                        {job.packages && job.packages.length > 0 && (
                            <div className="space-y-1.5">
                                <p className="text-[12px] text-muted-foreground">
                                    生成パッケージ
                                </p>
                                <div className="flex flex-wrap gap-1.5">
                                    {job.packages.map((p) => (
                                        <span
                                            key={p}
                                            className="rounded-sm bg-muted px-2 py-0.5 font-mono text-[12px] text-foreground"
                                        >
                                            {p}
                                        </span>
                                    ))}
                                </div>
                            </div>
                        )}

                        {job.err && (
                            <p className="rounded-sm border border-destructive/40 bg-destructive/10 px-3 py-2 text-[13px] text-destructive">
                                {job.err}
                            </p>
                        )}

                        <div className="space-y-2">
                            <div className="flex items-center gap-1.5 text-[12px] text-muted-foreground">
                                <ScrollText className="h-3.5 w-3.5" />
                                ライブログ
                            </div>
                            <pre
                                ref={logRef}
                                className="h-80 overflow-auto rounded-md border border-border bg-muted p-3 font-mono text-[12px] leading-relaxed whitespace-pre-wrap"
                            >
                                {logLines.length > 0
                                    ? logLines.join("\n")
                                    : "ログを待機しています..."}
                            </pre>
                        </div>

                        {isActive(job.status) && (
                            <AuthGate fallback={null}>
                                <div className="flex justify-end">
                                    <Button
                                        variant="destructive"
                                        size="sm"
                                        onClick={handleCancel}
                                        disabled={cancelling}
                                    >
                                        {cancelling ? (
                                            <Loader2 className="mr-2 h-4 w-4 animate-spin" />
                                        ) : (
                                            <Ban className="mr-2 h-4 w-4" />
                                        )}
                                        キャンセル
                                    </Button>
                                </div>
                            </AuthGate>
                        )}
                    </>
                )}
            </DialogContent>
        </Dialog>
    );
}
