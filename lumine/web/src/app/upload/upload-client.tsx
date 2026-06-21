"use client";

import {
    CheckCircle2,
    FileArchive,
    FileKey,
    Upload,
    XCircle,
} from "lucide-react";
import { useState } from "react";
import { useAuth } from "@/components/auth-provider";
import { AuthGate } from "@/components/auth-gate";
import { useAPIClient } from "@/components/lumine-provider";
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

const PKG_NAME_RE = /\.pkg\.tar\.(zst|xz|gz)$/i;

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
    const { toast } = useToast();
    const { username, password } = useAuth();
    const { selectedRepo, setSelectedRepo, repos } = useRepoArch();

    const [packageFile, setPackageFile] = useState<File | null>(null);
    const [packageError, setPackageError] = useState<string | null>(null);
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
        if (file && !PKG_NAME_RE.test(file.name)) {
            setPackageError(
                ".pkg.tar.zst / .pkg.tar.xz / .pkg.tar.gz 形式のファイルを選択してください",
            );
        } else {
            setPackageError(null);
        }
    };

    const handleSignatureFileChange = (
        e: React.ChangeEvent<HTMLInputElement>,
    ) => {
        setSignatureFile(e.target.files?.[0] ?? null);
    };

    const canSubmit =
        !uploading && !!selectedRepo && !!packageFile && !packageError;

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
                username || undefined,
                password || undefined,
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
            setPackageError(null);
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
                    accept=".pkg.tar.zst,.pkg.tar.xz,.pkg.tar.gz"
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
