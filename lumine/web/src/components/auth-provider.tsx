"use client";

import type { ReactNode } from "react";
import { createContext, useCallback, useContext, useState } from "react";

interface AuthContextType {
    isAuthenticated: boolean;
    githubLogin: string | null;
    githubId: number | null;
    meLoading: boolean;
    setMe: (m: { authenticated: boolean; login?: string; id?: number }) => void;
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

    return (
        <AuthContext.Provider
            value={{
                isAuthenticated,
                githubLogin,
                githubId,
                meLoading,
                setMe,
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
