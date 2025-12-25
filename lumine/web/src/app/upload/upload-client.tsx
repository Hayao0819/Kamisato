"use client";

import { Button } from "@/components/ui/button";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import {
    Select,
    SelectContent,
    SelectItem,
    SelectTrigger,
    SelectValue,
} from "@/components/ui/select";
import { useToast } from "@/hooks/use-toast";
import { useAPIClient } from "@/components/lumine-provider";
import { useAuth } from "@/components/auth-provider";
import { Upload, FileArchive, FileKey, AlertCircle } from "lucide-react";
import { useEffect, useState } from "react";
import { Alert, AlertDescription, AlertTitle } from "@/components/ui/alert";

export function UploadPageClient() {
    const api = useAPIClient();
    const { toast } = useToast();
    const { isAuthenticated, username, password } = useAuth();

    const [repos, setRepos] = useState<string[]>([]);
    const [selectedRepo, setSelectedRepo] = useState<string>("");
    const [packageFile, setPackageFile] = useState<File | null>(null);
    const [signatureFile, setSignatureFile] = useState<File | null>(null);
    const [uploading, setUploading] = useState(false);

    useEffect(() => {
        const loadRepos = async () => {
            try {
                const data = await api.fetchRepos();
                setRepos(data.repos || []);
                if (data.repos && data.repos.length > 0) {
                    setSelectedRepo(data.repos[0]);
                }
            } catch (error) {
                toast({
                    title: "エラー",
                    description: "リポジトリ一覧の取得に失敗しました",
                    variant: "destructive",
                });
            }
        };
        loadRepos();
    }, [api, toast]);

    const handlePackageFileChange = (e: React.ChangeEvent<HTMLInputElement>) => {
        if (e.target.files && e.target.files.length > 0) {
            setPackageFile(e.target.files[0]);
        }
    };

    const handleSignatureFileChange = (e: React.ChangeEvent<HTMLInputElement>) => {
        if (e.target.files && e.target.files.length > 0) {
            setSignatureFile(e.target.files[0]);
        }
    };

    const handleUpload = async (e: React.FormEvent) => {
        e.preventDefault();

        if (!packageFile) {
            toast({
                title: "エラー",
                description: "パッケージファイルを選択してください",
                variant: "destructive",
            });
            return;
        }

        if (!selectedRepo) {
            toast({
                title: "エラー",
                description: "リポジトリを選択してください",
                variant: "destructive",
            });
            return;
        }

        setUploading(true);

        try {
            const result = await api.uploadPackage(
                selectedRepo,
                packageFile,
                signatureFile,
                username || undefined,
                password || undefined,
            );

            toast({
                title: "成功",
                description: result || "パッケージをアップロードしました",
            });

            // Reset form
            setPackageFile(null);
            setSignatureFile(null);
            // Reset file inputs
            const packageInput = document.getElementById("package-file") as HTMLInputElement;
            const signatureInput = document.getElementById("signature-file") as HTMLInputElement;
            if (packageInput) packageInput.value = "";
            if (signatureInput) signatureInput.value = "";
        } catch (error) {
            toast({
                title: "エラー",
                description: error instanceof Error ? error.message : "アップロードに失敗しました",
                variant: "destructive",
            });
        } finally {
            setUploading(false);
        }
    };

    return (
        <div className="container mx-auto py-8 px-4 max-w-2xl">
            {!isAuthenticated && (
                <Alert className="mb-6">
                    <AlertCircle className="h-4 w-4" />
                    <AlertTitle>認証が必要な場合があります</AlertTitle>
                    <AlertDescription>
                        サーバー設定により、パッケージのアップロードに認証が必要な場合があります。
                        ヘッダーのログインボタンから認証情報を設定してください。
                    </AlertDescription>
                </Alert>
            )}

            <Card>
                <CardHeader>
                    <CardTitle className="flex items-center gap-2">
                        <Upload className="w-6 h-6" />
                        パッケージアップロード
                    </CardTitle>
                    <CardDescription>
                        パッケージバイナリをリポジトリにアップロードします
                    </CardDescription>
                </CardHeader>
                <CardContent>
                    <form onSubmit={handleUpload} className="space-y-6">
                        {/* Repository Selection */}
                        <div className="space-y-2">
                            <Label htmlFor="repository">リポジトリ</Label>
                            <Select value={selectedRepo} onValueChange={setSelectedRepo}>
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

                        {/* Package File */}
                        <div className="space-y-2">
                            <Label htmlFor="package-file" className="flex items-center gap-2">
                                <FileArchive className="w-4 h-4" />
                                パッケージファイル
                                <span className="text-red-500">*</span>
                            </Label>
                            <Input
                                id="package-file"
                                type="file"
                                accept=".pkg.tar.zst,.pkg.tar.xz,.pkg.tar.gz"
                                onChange={handlePackageFileChange}
                                required
                            />
                            {packageFile && (
                                <p className="text-sm text-muted-foreground">
                                    選択済み: {packageFile.name} ({(packageFile.size / 1024 / 1024).toFixed(2)} MB)
                                </p>
                            )}
                        </div>

                        {/* Signature File */}
                        <div className="space-y-2">
                            <Label htmlFor="signature-file" className="flex items-center gap-2">
                                <FileKey className="w-4 h-4" />
                                署名ファイル (オプション)
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

                        {/* Submit Button */}
                        <Button
                            type="submit"
                            className="w-full"
                            disabled={uploading || !packageFile || !selectedRepo}
                        >
                            {uploading ? (
                                <>
                                    <span className="animate-spin mr-2">⏳</span>
                                    アップロード中...
                                </>
                            ) : (
                                <>
                                    <Upload className="w-4 h-4 mr-2" />
                                    アップロード
                                </>
                            )}
                        </Button>
                    </form>
                </CardContent>
            </Card>

            {/* Information */}
            <Card className="mt-6">
                <CardHeader>
                    <CardTitle className="text-base">注意事項</CardTitle>
                </CardHeader>
                <CardContent className="text-sm space-y-2 text-muted-foreground">
                    <ul className="list-disc list-inside space-y-1">
                        <li>パッケージファイルは .pkg.tar.zst, .pkg.tar.xz, .pkg.tar.gz 形式に対応しています</li>
                        <li>署名ファイルは .sig 形式です（サーバー設定により必須の場合があります）</li>
                        <li>認証情報は Basic 認証で送信されます</li>
                        <li>アップロード後、パッケージデータベースが自動的に更新されます</li>
                        <li>ログイン済みの場合、保存された認証情報が自動的に使用されます</li>
                    </ul>
                </CardContent>
            </Card>
        </div>
    );
}
