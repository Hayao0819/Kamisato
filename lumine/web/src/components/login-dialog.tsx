"use client";

import { Github, LogOut, User } from "lucide-react";
import { useState } from "react";
import { useAuth } from "@/components/auth-provider";
import { useAPIClient } from "@/components/lumine-provider";
import { Button } from "@/components/ui/button";
import {
    Dialog,
    DialogContent,
    DialogDescription,
    DialogHeader,
    DialogTitle,
    DialogTrigger,
} from "@/components/ui/dialog";
import { useToast } from "@/hooks/use-toast";

export function LoginDialog() {
    const { isAuthenticated, githubLogin, setMe } = useAuth();
    const api = useAPIClient();
    const { toast } = useToast();
    const [open, setOpen] = useState(false);

    const handleSignOut = async () => {
        await api.signOut();
        setMe({ authenticated: false });
        toast({
            title: "ログアウト",
            description: "ログアウトしました",
        });
        setOpen(false);
    };

    const handleSignIn = async () => {
        try {
            await api.signIn();
        } catch (e) {
            toast({
                title: "ログイン失敗",
                description:
                    e instanceof Error ? e.message : "ログインに失敗しました",
                variant: "destructive",
            });
            return;
        }
        // Cookie mode has navigated away by now; only bearer mode reaches here,
        // where the token is set and the session can be refreshed in place.
        const me = await api.fetchMe();
        setMe(me);
        setOpen(false);
    };

    if (isAuthenticated) {
        return (
            <Dialog open={open} onOpenChange={setOpen}>
                <DialogTrigger asChild>
                    <Button
                        variant="outline"
                        className="flex items-center gap-2"
                    >
                        <User className="w-4 h-4" />
                        <span className="hidden sm:inline">
                            {githubLogin ?? "ログイン中"}
                        </span>
                    </Button>
                </DialogTrigger>
                <DialogContent>
                    <DialogHeader>
                        <DialogTitle>ログイン中</DialogTitle>
                        <DialogDescription>
                            GitHub アカウントでログインしています
                        </DialogDescription>
                    </DialogHeader>
                    <div className="space-y-4">
                        <div className="p-4 border rounded-lg bg-muted/50">
                            <div className="flex items-center gap-2 text-sm">
                                <Github className="w-4 h-4" />
                                <span className="font-semibold">GitHub:</span>
                                <span>{githubLogin ?? "—"}</span>
                            </div>
                        </div>
                        <Button
                            onClick={handleSignOut}
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
                    <Github className="w-4 h-4" />
                    <span className="hidden sm:inline">ログイン</span>
                </Button>
            </DialogTrigger>
            <DialogContent>
                <DialogHeader>
                    <DialogTitle>ログイン</DialogTitle>
                    <DialogDescription>
                        パッケージのアップロードやビルドには認証が必要です
                    </DialogDescription>
                </DialogHeader>
                <div className="space-y-4">
                    <Button onClick={handleSignIn} className="w-full">
                        <Github className="w-4 h-4 mr-2" />
                        GitHub でログイン
                    </Button>
                    <p className="text-xs text-muted-foreground">
                        GitHub
                        の認証ページへ移動します。許可されたアカウントのみ操作できます。
                    </p>
                </div>
            </DialogContent>
        </Dialog>
    );
}
