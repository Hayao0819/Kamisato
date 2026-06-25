"use client";

import { Lock } from "lucide-react";
import { type ReactNode, useEffect, useState } from "react";
import { useAuth } from "@/components/auth-provider";
import { LoginDialog } from "@/components/login-dialog";

// Gate on mount so the first client render matches SSR (avoids hydration mismatch).
export function useCanMutate(): boolean {
    const { meLoading, isAuthenticated } = useAuth();
    const [mounted, setMounted] = useState(false);
    useEffect(() => setMounted(true), []);
    if (!mounted || meLoading) return false;
    return isAuthenticated;
}

export function LoginPrompt() {
    return (
        <div className="rounded-md border bg-card p-6 text-card-foreground">
            <div className="flex items-start gap-3">
                <span className="mt-0.5 flex h-9 w-9 shrink-0 items-center justify-center rounded-sm bg-muted text-muted-foreground">
                    <Lock className="h-4 w-4" />
                </span>
                <div className="space-y-3">
                    <div className="space-y-1">
                        <p className="font-medium">
                            この操作にはログインが必要です
                        </p>
                        <p className="text-sm text-muted-foreground">
                            続けるにはログインしてください。
                        </p>
                    </div>
                    <LoginDialog />
                </div>
            </div>
        </div>
    );
}

export function AuthGate({
    children,
    fallback,
}: {
    children: ReactNode;
    fallback?: ReactNode;
}) {
    const canMutate = useCanMutate();
    if (canMutate) return <>{children}</>;
    return <>{fallback ?? <LoginPrompt />}</>;
}
