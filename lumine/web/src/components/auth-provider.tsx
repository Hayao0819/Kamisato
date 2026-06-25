"use client";

import type { ReactNode } from "react";
import { createContext, useCallback, useContext, useState } from "react";

interface AuthContextType {
    isAuthenticated: boolean;
    githubLogin: string | null;
    githubId: number | null;
    meLoading: boolean;
    setMe: (m: { authenticated: boolean; login?: string; id?: number }) => void;
    signIn: () => void;
    signOut: () => Promise<void>;
}

const AuthContext = createContext<AuthContextType | undefined>(undefined);

export function AuthProvider({ children }: { children: ReactNode }) {
    const [isAuthenticated, setIsAuthenticated] = useState(false);
    const [githubLogin, setGithubLogin] = useState<string | null>(null);
    const [githubId, setGithubId] = useState<number | null>(null);
    const [meLoading, setMeLoading] = useState(true);

    const setMe = useCallback(
        (m: { authenticated: boolean; login?: string; id?: number }) => {
            setIsAuthenticated(m.authenticated);
            setGithubLogin(m.authenticated ? (m.login ?? null) : null);
            setGithubId(m.authenticated ? (m.id ?? null) : null);
            setMeLoading(false);
        },
        [],
    );

    // The 302-to-GitHub redirect and the Lax session cookie only work on a
    // top-level navigation, never a fetch. The URL is relative so it resolves
    // same-origin in both dev (next rewrite) and prod (lumine BFF).
    const signIn = useCallback(() => {
        window.location.assign("/api/unstable/auth/github/login");
    }, []);

    const signOut = useCallback(async () => {
        try {
            await fetch("/api/unstable/auth/logout", { method: "POST" });
        } catch {
            // ignore: logout is best-effort
        }
        setIsAuthenticated(false);
        setGithubLogin(null);
        setGithubId(null);
    }, []);

    return (
        <AuthContext.Provider
            value={{
                isAuthenticated,
                githubLogin,
                githubId,
                meLoading,
                setMe,
                signIn,
                signOut,
            }}
        >
            {children}
        </AuthContext.Provider>
    );
}

export function useAuth() {
    const context = useContext(AuthContext);
    if (context === undefined) {
        throw new Error("useAuth must be used within an AuthProvider");
    }
    return context;
}
