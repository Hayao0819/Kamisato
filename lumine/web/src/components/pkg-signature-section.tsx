"use client";

import { Download, ShieldCheck, ShieldQuestion } from "lucide-react";
import { useEffect, useState } from "react";
import { useAPIClient } from "@/components/lumine-provider";
import type { PackageSignature } from "@/lib/types";

interface PkgSignatureSectionProps {
    repo: string;
    arch: string;
    pkgname: string;
}

const formatSignedAt = (unixSec: number): string =>
    new Intl.DateTimeFormat("ja-JP", {
        dateStyle: "medium",
        timeStyle: "short",
    }).format(new Date(unixSec * 1000));

export function PkgSignatureSection({
    repo,
    arch,
    pkgname,
}: PkgSignatureSectionProps) {
    const api = useAPIClient();

    const [signature, setSignature] = useState<PackageSignature | null>(null);
    const [loading, setLoading] = useState(true);
    const [error, setError] = useState<string | null>(null);

    useEffect(() => {
        if (!api.endpoints.executable) return;
        let ignore = false;
        const run = async () => {
            setLoading(true);
            setError(null);
            try {
                const data = await api.fetchPkgSignature(repo, arch, pkgname);
                if (!ignore) setSignature(data);
            } catch (e: unknown) {
                if (!ignore)
                    setError(
                        e instanceof Error
                            ? e.message
                            : "署名情報の取得に失敗しました",
                    );
            } finally {
                if (!ignore) setLoading(false);
            }
        };
        run();
        return () => {
            ignore = true;
        };
    }, [repo, arch, pkgname, api]);

    return (
        <section className="rounded-sm border border-border bg-card">
            <h2 className="border-b border-border px-4 py-2.5 text-[14px] font-semibold text-muted-foreground">
                署名
            </h2>
            <div className="space-y-3 px-4 py-3 text-[15px]">
                {loading ? (
                    <p className="text-muted-foreground">
                        署名情報を読み込み中...
                    </p>
                ) : error ? (
                    <p className="text-destructive">{error}</p>
                ) : !signature?.present ? (
                    <p className="flex items-center gap-2 text-muted-foreground">
                        <ShieldQuestion className="h-4 w-4" />
                        このパッケージには署名がありません
                    </p>
                ) : (
                    <>
                        <p className="flex items-center gap-2">
                            <ShieldCheck className="h-4 w-4 text-emerald-600 dark:text-emerald-400" />
                            署名ファイルが公開されています
                        </p>
                        <dl className="grid grid-cols-[auto_1fr] gap-x-4 gap-y-1.5">
                            {signature.fingerprint ? (
                                <>
                                    <dt className="text-muted-foreground">
                                        フィンガープリント
                                    </dt>
                                    <dd className="break-all font-mono text-[14px]">
                                        {signature.fingerprint}
                                    </dd>
                                </>
                            ) : signature.key_id ? (
                                <>
                                    <dt className="text-muted-foreground">
                                        鍵 ID
                                    </dt>
                                    <dd className="font-mono text-[14px]">
                                        {signature.key_id}
                                    </dd>
                                </>
                            ) : null}
                            {signature.created_at ? (
                                <>
                                    <dt className="text-muted-foreground">
                                        署名日時
                                    </dt>
                                    <dd className="tabular-nums">
                                        {formatSignedAt(signature.created_at)}
                                    </dd>
                                </>
                            ) : null}
                            {signature.pubkey_algo && (
                                <>
                                    <dt className="text-muted-foreground">
                                        アルゴリズム
                                    </dt>
                                    <dd>
                                        {signature.pubkey_algo}
                                        {signature.hash &&
                                            ` / ${signature.hash}`}
                                    </dd>
                                </>
                            )}
                        </dl>
                        {signature.filename && (
                            <a
                                href={api.endpoints.repoFile(
                                    repo,
                                    arch,
                                    signature.filename,
                                )}
                                target="_blank"
                                rel="noopener noreferrer"
                                className="inline-flex h-8 items-center gap-1.5 rounded-sm border border-border px-3 text-[14px] hover:bg-muted"
                            >
                                <Download className="h-3.5 w-3.5" />
                                .sig をダウンロード
                            </a>
                        )}
                    </>
                )}
            </div>
        </section>
    );
}
