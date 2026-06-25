"use client";
import {
    createContext,
    type ReactNode,
    useContext,
    useEffect,
    useState,
} from "react";
import { APIClient } from "@/lib/api";
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
    const { setMe } = useAuth();

    useEffect(() => {
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
        }

        if (!client.endpoints.executable) {
            setMe({ authenticated: false });
            return;
        }

        client
            .fetchMe()
            .then((me) => {
                setMe(me);
            })
            .catch((err) => {
                console.error("Failed to fetch session:", err);
                setMe({ authenticated: false });
            });
    }, [client, setMe]);

    return (
        <APIClientContext.Provider value={client}>
            {children}
        </APIClientContext.Provider>
    );
}
