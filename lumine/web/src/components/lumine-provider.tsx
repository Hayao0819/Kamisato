"use client";
import {
    createContext,
    type ReactNode,
    useContext,
    useEffect,
    useState,
} from "react";
import { APIClient, defaultFeatures, type Features } from "@/lib/api";
import { useAuth } from "./auth-provider";

const APIClientContext = createContext<APIClient | null>(null);
const FeaturesContext = createContext<Features>(defaultFeatures);

export function useAPIClient() {
    const ctx = useContext(APIClientContext);
    if (!ctx)
        throw new Error("useAPIClient must be used within a LumineProvider");
    return ctx;
}

// Optional features advertised by ayato, fetched once at startup. Everything
// stays off until the fetch resolves so the UI never flashes unavailable views.
export function useFeatures() {
    return useContext(FeaturesContext);
}

export default function LumineProvider({
    children,
}: Readonly<{ children: ReactNode }>) {
    const [client, setClient] = useState<APIClient>(APIClient.fallback);
    const [features, setFeatures] = useState<Features>(defaultFeatures);
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
            setFeatures(defaultFeatures);
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

        client
            .features()
            .then(setFeatures)
            .catch((err) => {
                console.error("Failed to fetch features:", err);
                setFeatures(defaultFeatures);
            });
    }, [client, setMe]);

    return (
        <APIClientContext.Provider value={client}>
            <FeaturesContext.Provider value={features}>
                {children}
            </FeaturesContext.Provider>
        </APIClientContext.Provider>
    );
}
