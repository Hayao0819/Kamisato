"use client";

import { BugIcon } from "lucide-react";
import type React from "react";
import { useState } from "react";
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
import type { PackageInfo } from "@/lib/types";

interface BugReportDialogProps {
    packageInfo: PackageInfo;
}

export function BugReportDialog({ packageInfo }: BugReportDialogProps) {
    const [open, setOpen] = useState(false);
    const { toast } = useToast();

    const handleSubmit = (e: React.FormEvent) => {
        e.preventDefault();
        // ここでバグ報告を送信する処理を実装（モックなので実際には何もしない）
        toast({
            title: "バグ報告を送信しました",
            description: `${packageInfo.pkgname} のバグ報告ありがとうございます。開発チームが確認します。`,
        });
        setOpen(false);
    };

    return (
        <Dialog open={open} onOpenChange={setOpen}>
            <DialogTrigger asChild>
                <Button
                    variant="outline"
                    size="sm"
                    className="text-xs sm:text-sm"
                >
                    <BugIcon className="h-3 w-3 sm:h-4 sm:w-4 mr-1 sm:mr-2" />
                    バグ報告
                </Button>
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
                                <Input id="name" />
                            </div>
                        </div>
                        <div className="grid grid-cols-1 sm:grid-cols-4 items-start sm:items-center gap-2 sm:gap-4">
                            <Label htmlFor="email" className="sm:text-right">
                                メール
                            </Label>
                            <div className="sm:col-span-3">
                                <Input id="email" type="email" />
                            </div>
                        </div>
                        <div className="grid grid-cols-1 sm:grid-cols-4 items-start sm:items-center gap-2 sm:gap-4">
                            <Label htmlFor="severity" className="sm:text-right">
                                重要度
                            </Label>
                            <div className="sm:col-span-3">
                                <Select defaultValue="medium">
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
                                    placeholder="バグの詳細を記入してください"
                                />
                            </div>
                        </div>
                    </div>
                    <DialogFooter className="flex-col sm:flex-row gap-2">
                        <Button type="submit" className="w-full sm:w-auto">
                            送信
                        </Button>
                    </DialogFooter>
                </form>
            </DialogContent>
        </Dialog>
    );
}
