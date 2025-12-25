"use client";
import { APIClient } from "@/lib/api";
import { type ReactNode, useContext, useEffect, useState } from "react";
import { createContext } from "react";
import { useAuth } from "./auth-provider";

const APIClientContext = createContext<APIClient | null>(null);

export function useAPIClient() {
    const ctx = useContext(APIClientContext);
    if (!ctx)
        throw new Error("useAPIClient must be used within a LumineProvider");
    return ctx;
}

export default function LumineProvider({
    children,
}: Readonly<{ children: ReactNode }>) {
    const [client, setClient] = useState<APIClient>(APIClient.fallback);
    const { setAuthRequired, setAuthRequiredLoading } = useAuth();

    console.log("LumineProvider initialized with client:", client);

    useEffect(() => {
        console.log("Initializing APIClient...");
        APIClient.init()
            .then((client) => {
                setClient(client);
            })
            .catch((err) => {
                console.error("APIClient initialization failed:", err);
            });
    }, []);

    useEffect(() => {
        if (client.lumineEnv.FALLBACK) {
            console.warn(
                "APIClient is using fallback mode. Some features may not work as expected.",
            );
        } else {
            console.log(
                "APIClient initialized successfully with environment:",
                client.lumineEnv,
            );
        }

        // Check auth requirement
        client.fetchAuthRequired()
            .then((result) => {
                setAuthRequired(result.required);
                setAuthRequiredLoading(false);
            })
            .catch((err) => {
                console.error("Failed to check auth requirement:", err);
                setAuthRequired(false);
                setAuthRequiredLoading(false);
            });
    }, [client, setAuthRequired, setAuthRequiredLoading]);

    return (
        <APIClientContext.Provider value={client}>
            {children}
        </APIClientContext.Provider>
    );
}
