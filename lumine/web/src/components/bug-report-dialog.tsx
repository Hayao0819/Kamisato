"use client";

import { BugIcon, Loader2 } from "lucide-react";
import type React from "react";
import { useRef, useState } from "react";
import ReCAPTCHA from "react-google-recaptcha";
import { useAPIClient, useFeatures } from "@/components/lumine-provider";
import { Button } from "@/components/ui/button";
import {
    Dialog,
    DialogContent,
    DialogDescription,
    DialogFooter,
    DialogHeader,
    DialogTitle,
    DialogTrigger,
} from "@/components/ui/dialog";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import {
    Select,
    SelectContent,
    SelectItem,
    SelectTrigger,
    SelectValue,
} from "@/components/ui/select";
import { Textarea } from "@/components/ui/textarea";
import { useToast } from "@/hooks/use-toast";
import type { Severity } from "@/lib/api";
import type { PackageInfo } from "@/lib/types";

interface BugReportDialogProps {
    packageInfo: PackageInfo;
    repo: string;
    trigger?: React.ReactNode;
}

export function BugReportDialog({
    packageInfo,
    repo,
    trigger,
}: BugReportDialogProps) {
    const api = useAPIClient();
    const features = useFeatures();
    const { toast } = useToast();
    const [open, setOpen] = useState(false);
    const [submitting, setSubmitting] = useState(false);

    const [name, setName] = useState("");
    const [email, setEmail] = useState("");
    const [severity, setSeverity] = useState<Severity>("medium");
    const [description, setDescription] = useState("");
    const [token, setToken] = useState<string | null>(null);
    const recaptchaRef = useRef<ReCAPTCHA>(null);

    const siteKey = features.recaptcha_site_key;

    const reset = () => {
        setName("");
        setEmail("");
        setSeverity("medium");
        setDescription("");
        setToken(null);
        recaptchaRef.current?.reset();
    };

    const handleSubmit = async (e: React.FormEvent) => {
        e.preventDefault();
        if (!description.trim()) {
            toast({
                title: "詳細を入力してください",
                variant: "destructive",
            });
            return;
        }
        if (siteKey && !token) {
            toast({
                title: "reCAPTCHA を完了してください",
                variant: "destructive",
            });
            return;
        }

        setSubmitting(true);
        try {
            const { url } = await api.submitBugReport({
                pkgname: packageInfo.pkgname,
                pkgver: packageInfo.pkgver,
                repo,
                arch: packageInfo.arch,
                name: name.trim(),
                email: email.trim(),
                severity,
                description: description.trim(),
                recaptcha_token: token ?? "",
            });
            toast({
                title: "バグ報告を送信しました",
                description: (
                    <a
                        href={url}
                        target="_blank"
                        rel="noopener noreferrer"
                        className="underline"
                    >
                        作成された Issue を開く
                    </a>
                ),
            });
            setOpen(false);
            reset();
        } catch (err) {
            toast({
                title: "バグ報告の送信に失敗しました",
                description:
                    err instanceof Error ? err.message : "送信に失敗しました",
                variant: "destructive",
            });
        } finally {
            setSubmitting(false);
        }
    };

    return (
        <Dialog open={open} onOpenChange={setOpen}>
            <DialogTrigger asChild>
                {trigger ?? (
                    <Button
                        variant="outline"
                        size="sm"
                        className="text-xs sm:text-sm"
                    >
                        <BugIcon className="h-3 w-3 sm:h-4 sm:w-4 mr-1 sm:mr-2" />
                        バグ報告
                    </Button>
                )}
            </DialogTrigger>
            <DialogContent className="sm:max-w-[425px] max-w-[90vw] w-full">
                <DialogHeader>
                    <DialogTitle>バグ報告</DialogTitle>
                    <DialogDescription>
                        {packageInfo.pkgname} ({packageInfo.pkgver})
                        に関するバグを報告します。
                    </DialogDescription>
                </DialogHeader>
                <form onSubmit={handleSubmit}>
                    <div className="grid gap-4 py-4">
                        <div className="grid grid-cols-1 sm:grid-cols-4 items-start sm:items-center gap-2 sm:gap-4">
                            <Label htmlFor="name" className="sm:text-right">
                                お名前
                            </Label>
                            <div className="sm:col-span-3">
                                <Input
                                    id="name"
                                    value={name}
                                    onChange={(e) => setName(e.target.value)}
                                />
                            </div>
                        </div>
                        <div className="grid grid-cols-1 sm:grid-cols-4 items-start sm:items-center gap-2 sm:gap-4">
                            <Label htmlFor="email" className="sm:text-right">
                                メール
                            </Label>
                            <div className="sm:col-span-3">
                                <Input
                                    id="email"
                                    type="email"
                                    value={email}
                                    onChange={(e) => setEmail(e.target.value)}
                                />
                            </div>
                        </div>
                        <div className="grid grid-cols-1 sm:grid-cols-4 items-start sm:items-center gap-2 sm:gap-4">
                            <Label htmlFor="severity" className="sm:text-right">
                                重要度
                            </Label>
                            <div className="sm:col-span-3">
                                <Select
                                    value={severity}
                                    onValueChange={(v) =>
                                        setSeverity(v as Severity)
                                    }
                                >
                                    <SelectTrigger id="severity">
                                        <SelectValue placeholder="重要度を選択" />
                                    </SelectTrigger>
                                    <SelectContent>
                                        <SelectItem value="critical">
                                            致命的
                                        </SelectItem>
                                        <SelectItem value="high">高</SelectItem>
                                        <SelectItem value="medium">
                                            中
                                        </SelectItem>
                                        <SelectItem value="low">低</SelectItem>
                                    </SelectContent>
                                </Select>
                            </div>
                        </div>
                        <div className="grid grid-cols-1 sm:grid-cols-4 items-start gap-2 sm:gap-4">
                            <Label
                                htmlFor="description"
                                className="sm:text-right pt-2"
                            >
                                詳細
                            </Label>
                            <div className="sm:col-span-3">
                                <Textarea
                                    id="description"
                                    rows={5}
                                    value={description}
                                    onChange={(e) =>
                                        setDescription(e.target.value)
                                    }
                                    placeholder="バグの詳細を記入してください"
                                />
                            </div>
                        </div>
                        {siteKey && (
                            <div className="flex justify-center sm:justify-end">
                                <ReCAPTCHA
                                    ref={recaptchaRef}
                                    sitekey={siteKey}
                                    onChange={setToken}
                                />
                            </div>
                        )}
                    </div>
                    <DialogFooter className="flex-col sm:flex-row gap-2">
                        <Button
                            type="submit"
                            className="w-full sm:w-auto"
                            disabled={submitting}
                        >
                            {submitting ? (
                                <>
                                    <Loader2 className="mr-2 h-4 w-4 animate-spin" />
                                    送信中...
                                </>
                            ) : (
                                "送信"
                            )}
                        </Button>
                    </DialogFooter>
                </form>
            </DialogContent>
        </Dialog>
    );
}
