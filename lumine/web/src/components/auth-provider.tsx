"use client";

import { createContext, useContext, useState, useEffect, ReactNode } from "react";
import type { APIClient } from "@/lib/api";

interface AuthContextType {
    isAuthenticated: boolean;
    username: string | null;
    password: string | null;
    authRequired: boolean;
    authRequiredLoading: boolean;
    login: (username: string, password: string) => void;
    logout: () => void;
    setAuthRequired: (required: boolean) => void;
    setAuthRequiredLoading: (loading: boolean) => void;
}

const AuthContext = createContext<AuthContextType | undefined>(undefined);

const AUTH_STORAGE_KEY = "lumine_auth_credentials";

export function AuthProvider({ children }: { children: ReactNode }) {
    const [isAuthenticated, setIsAuthenticated] = useState(false);
    const [username, setUsername] = useState<string | null>(null);
    const [password, setPassword] = useState<string | null>(null);
    const [authRequired, setAuthRequired] = useState(false);
    const [authRequiredLoading, setAuthRequiredLoading] = useState(true);

    // Load credentials from localStorage on mount
    useEffect(() => {
        if (typeof window !== "undefined") {
            const stored = localStorage.getItem(AUTH_STORAGE_KEY);
            if (stored) {
                try {
                    const { username: storedUsername, password: storedPassword } = JSON.parse(stored);
                    if (storedUsername && storedPassword) {
                        setUsername(storedUsername);
                        setPassword(storedPassword);
                        setIsAuthenticated(true);
                    }
                } catch (e) {
                    console.error("Failed to parse stored credentials", e);
                }
            }
        }
    }, []);

    const login = (newUsername: string, newPassword: string) => {
        setUsername(newUsername);
        setPassword(newPassword);
        setIsAuthenticated(true);

        // Store credentials in localStorage
        if (typeof window !== "undefined") {
            localStorage.setItem(
                AUTH_STORAGE_KEY,
                JSON.stringify({ username: newUsername, password: newPassword })
            );
        }
    };

    const logout = () => {
        setUsername(null);
        setPassword(null);
        setIsAuthenticated(false);

        // Remove credentials from localStorage
        if (typeof window !== "undefined") {
            localStorage.removeItem(AUTH_STORAGE_KEY);
        }
    };

    return (
        <AuthContext.Provider
            value={{
                isAuthenticated,
                username,
                password,
                authRequired,
                authRequiredLoading,
                login,
                logout,
                setAuthRequired,
                setAuthRequiredLoading
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
