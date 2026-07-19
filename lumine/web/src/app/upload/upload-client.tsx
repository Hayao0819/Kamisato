"use client";

import {
    CheckCircle2,
    FileArchive,
    FileKey,
    Upload,
    XCircle,
} from "lucide-react";
import { useState } from "react";
import { AuthGate } from "@/components/auth-gate";
import { useAPIClient, useFeatures } from "@/components/lumine-provider";
import { PageContainer } from "@/components/page-container";
import { PageHeader } from "@/components/page-header";
import { Alert, AlertDescription, AlertTitle } from "@/components/ui/alert";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Progress } from "@/components/ui/progress";
import {
    Select,
    SelectContent,
    SelectItem,
    SelectTrigger,
    SelectValue,
} from "@/components/ui/select";
import { useRepoArch } from "@/hooks/use-repo-arch";
import { useToast } from "@/hooks/use-toast";
import { isPackageArchive, packageFileAccept } from "@/lib/package-artifact";

export function UploadPageClient() {
    return (
        <PageContainer
            measure="form"
            header={
                <PageHeader
                    title="アップロード"
                    description="ビルド済みパッケージをリポジトリへ追加"
                />
            }
        >
            <AuthGate>
                <UploadForm />
            </AuthGate>
        </PageContainer>
    );
}

function UploadForm() {
    const api = useAPIClient();
    const features = useFeatures();
    const { toast } = useToast();
    const { selectedRepo, setSelectedRepo, repos } = useRepoArch();

    const [packageFile, setPackageFile] = useState<File | null>(null);
    const [signatureFile, setSignatureFile] = useState<File | null>(null);
    const [uploading, setUploading] = useState(false);
    const [uploadProgress, setUploadProgress] = useState(0);
    const [uploadStatus, setUploadStatus] = useState<
        "idle" | "success" | "error"
    >("idle");
    const [uploadMessage, setUploadMessage] = useState("");

    const handlePackageFileChange = (
        e: React.ChangeEvent<HTMLInputElement>,
    ) => {
        const file = e.target.files?.[0] ?? null;
        setPackageFile(file);
        setUploadStatus("idle");
    };

    const handleSignatureFileChange = (
        e: React.ChangeEvent<HTMLInputElement>,
    ) => {
        setSignatureFile(e.target.files?.[0] ?? null);
    };

    const packageError =
        packageFile &&
        !isPackageArchive(packageFile.name, features.package_archive_suffixes)
            ? `対応しているパッケージ形式を選択してください: ${features.package_archive_suffixes.join(" / ")}`
            : null;
    const signatureError =
        packageFile &&
        signatureFile &&
        signatureFile.name !== `${packageFile.name}.sig`
            ? `署名ファイル名は ${packageFile.name}.sig である必要があります`
            : null;
    const canSubmit =
        !uploading &&
        !!selectedRepo &&
        !!packageFile &&
        !packageError &&
        !signatureError;

    const handleUpload = async (e: React.FormEvent) => {
        e.preventDefault();
        if (!canSubmit || !packageFile || !selectedRepo) return;

        setUploading(true);
        setUploadStatus("idle");
        setUploadProgress(0);

        try {
            const result = await api.uploadPackageWithProgress(
                selectedRepo,
                packageFile,
                signatureFile,
                (progress) => {
                    setUploadProgress(progress);
                },
            );

            setUploadStatus("success");
            setUploadMessage(result || "パッケージをアップロードしました");
            toast({
                title: "成功",
                description: result || "パッケージをアップロードしました",
            });

            setPackageFile(null);
            setSignatureFile(null);
            const packageInput = document.getElementById(
                "package-file",
            ) as HTMLInputElement | null;
            const signatureInput = document.getElementById(
                "signature-file",
            ) as HTMLInputElement | null;
            if (packageInput) packageInput.value = "";
            if (signatureInput) signatureInput.value = "";
        } catch (error) {
            const message =
                error instanceof Error
                    ? error.message
                    : "アップロードに失敗しました";
            setUploadStatus("error");
            setUploadMessage(message);
            toast({
                title: "エラー",
                description: message,
                variant: "destructive",
            });
        } finally {
            setUploading(false);
        }
    };

    return (
        <form onSubmit={handleUpload} className="space-y-6">
            <div className="space-y-2">
                <Label htmlFor="repository">リポジトリ</Label>
                <Select
                    value={selectedRepo || undefined}
                    onValueChange={setSelectedRepo}
                >
                    <SelectTrigger id="repository">
                        <SelectValue placeholder="リポジトリを選択" />
                    </SelectTrigger>
                    <SelectContent>
                        {repos.map((repo) => (
                            <SelectItem key={repo} value={repo}>
                                {repo}
                            </SelectItem>
                        ))}
                    </SelectContent>
                </Select>
            </div>

            <div className="space-y-2">
                <Label
                    htmlFor="package-file"
                    className="flex items-center gap-2"
                >
                    <FileArchive className="h-4 w-4" />
                    パッケージファイル
                    <span className="text-destructive">*</span>
                </Label>
                <Input
                    id="package-file"
                    type="file"
                    accept={packageFileAccept(
                        features.package_archive_suffixes,
                    )}
                    onChange={handlePackageFileChange}
                />
                {packageError ? (
                    <p className="text-sm text-destructive">{packageError}</p>
                ) : packageFile ? (
                    <p className="text-sm text-muted-foreground">
                        選択済み: {packageFile.name} (
                        {(packageFile.size / 1024 / 1024).toFixed(2)} MB)
                    </p>
                ) : null}
            </div>

            <div className="space-y-2">
                <Label
                    htmlFor="signature-file"
                    className="flex items-center gap-2"
                >
                    <FileKey className="h-4 w-4" />
                    署名ファイル
                    <span className="text-muted-foreground">(任意)</span>
                </Label>
                <Input
                    id="signature-file"
                    type="file"
                    accept=".sig"
                    onChange={handleSignatureFileChange}
                />
                {signatureFile && (
                    <p className="text-sm text-muted-foreground">
                        選択済み: {signatureFile.name}
                    </p>
                )}
                {signatureError && (
                    <p className="text-sm text-destructive">{signatureError}</p>
                )}
            </div>

            {uploading && (
                <div className="space-y-2">
                    <div className="flex items-center justify-between text-sm">
                        <span className="text-muted-foreground">
                            アップロード中...
                        </span>
                        <span className="font-medium">
                            {uploadProgress.toFixed(0)}%
                        </span>
                    </div>
                    <Progress value={uploadProgress} />
                </div>
            )}

            {uploadStatus === "success" && (
                <Alert>
                    <CheckCircle2 className="h-4 w-4" />
                    <AlertTitle>アップロード成功</AlertTitle>
                    <AlertDescription>{uploadMessage}</AlertDescription>
                </Alert>
            )}

            {uploadStatus === "error" && (
                <Alert variant="destructive">
                    <XCircle className="h-4 w-4" />
                    <AlertTitle>アップロード失敗</AlertTitle>
                    <AlertDescription>{uploadMessage}</AlertDescription>
                </Alert>
            )}

            <Button type="submit" className="w-full" disabled={!canSubmit}>
                <Upload className="mr-2 h-4 w-4" />
                {uploading ? "アップロード中..." : "アップロード"}
            </Button>
        </form>
    );
}
