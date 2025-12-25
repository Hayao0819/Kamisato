"use client";

import { useState } from "react";
import { Button } from "@/components/ui/button";
import {
    Dialog,
    DialogContent,
    DialogDescription,
    DialogHeader,
    DialogTitle,
    DialogTrigger,
} from "@/components/ui/dialog";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { useAuth } from "@/components/auth-provider";
import { useToast } from "@/hooks/use-toast";
import { LogIn, LogOut, User, Lock } from "lucide-react";

export function LoginDialog() {
    const { isAuthenticated, username, login, logout } = useAuth();
    const { toast } = useToast();
    const [open, setOpen] = useState(false);
    const [inputUsername, setInputUsername] = useState("");
    const [inputPassword, setInputPassword] = useState("");

    const handleLogin = (e: React.FormEvent) => {
        e.preventDefault();

        if (!inputUsername || !inputPassword) {
            toast({
                title: "エラー",
                description: "ユーザー名とパスワードを入力してください",
                variant: "destructive",
            });
            return;
        }

        login(inputUsername, inputPassword);
        toast({
            title: "ログイン成功",
            description: `${inputUsername}としてログインしました`,
        });
        setOpen(false);
        setInputUsername("");
        setInputPassword("");
    };

    const handleLogout = () => {
        logout();
        toast({
            title: "ログアウト",
            description: "ログアウトしました",
        });
        setOpen(false);
    };

    if (isAuthenticated) {
        return (
            <Dialog open={open} onOpenChange={setOpen}>
                <DialogTrigger asChild>
                    <Button variant="outline" className="flex items-center gap-2">
                        <User className="w-4 h-4" />
                        <span className="hidden sm:inline">{username}</span>
                    </Button>
                </DialogTrigger>
                <DialogContent>
                    <DialogHeader>
                        <DialogTitle>ログイン中</DialogTitle>
                        <DialogDescription>
                            現在 {username} としてログインしています
                        </DialogDescription>
                    </DialogHeader>
                    <div className="space-y-4">
                        <div className="p-4 border rounded-lg bg-muted/50">
                            <div className="flex items-center gap-2 text-sm">
                                <User className="w-4 h-4" />
                                <span className="font-semibold">ユーザー名:</span>
                                <span>{username}</span>
                            </div>
                        </div>
                        <Button
                            onClick={handleLogout}
                            variant="destructive"
                            className="w-full"
                        >
                            <LogOut className="w-4 h-4 mr-2" />
                            ログアウト
                        </Button>
                    </div>
                </DialogContent>
            </Dialog>
        );
    }

    return (
        <Dialog open={open} onOpenChange={setOpen}>
            <DialogTrigger asChild>
                <Button variant="outline" className="flex items-center gap-2">
                    <LogIn className="w-4 h-4" />
                    <span className="hidden sm:inline">ログイン</span>
                </Button>
            </DialogTrigger>
            <DialogContent>
                <DialogHeader>
                    <DialogTitle>ログイン</DialogTitle>
                    <DialogDescription>
                        パッケージのアップロードには認証が必要です
                    </DialogDescription>
                </DialogHeader>
                <form onSubmit={handleLogin} className="space-y-4">
                    <div className="space-y-2">
                        <Label htmlFor="login-username" className="flex items-center gap-2">
                            <User className="w-4 h-4" />
                            ユーザー名
                        </Label>
                        <Input
                            id="login-username"
                            type="text"
                            value={inputUsername}
                            onChange={(e) => setInputUsername(e.target.value)}
                            placeholder="ユーザー名を入力"
                            required
                        />
                    </div>
                    <div className="space-y-2">
                        <Label htmlFor="login-password" className="flex items-center gap-2">
                            <Lock className="w-4 h-4" />
                            パスワード
                        </Label>
                        <Input
                            id="login-password"
                            type="password"
                            value={inputPassword}
                            onChange={(e) => setInputPassword(e.target.value)}
                            placeholder="パスワードを入力"
                            required
                        />
                    </div>
                    <Button type="submit" className="w-full">
                        <LogIn className="w-4 h-4 mr-2" />
                        ログイン
                    </Button>
                </form>
                <div className="text-xs text-muted-foreground">
                    <p>※ 認証情報はブラウザのローカルストレージに保存されます</p>
                    <p>※ サーバー設定により認証が不要な場合があります</p>
                </div>
            </DialogContent>
        </Dialog>
    );
}
