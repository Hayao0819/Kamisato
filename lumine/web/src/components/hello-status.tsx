"use client";

import { useEffect, useState } from "react";
import { useAPIClient } from "./lumine-provider";

export function HelloStatus() {
    const [status, setStatus] = useState<"success" | "error" | "loading">(
        "loading",
    );

    const api = useAPIClient();

    useEffect(() => {
        if (!api.endpoints.executable) return;
        const checkHello = async () => {
            setStatus("loading");
            try {
                const res = await api.fetchHello();
                if (res.ok) {
                    setStatus("success");
                } else {
                    setStatus("error");
                }
            } catch {
                setStatus("error");
            }
        };
        checkHello();
    }, [api.endpoints.executable]);

    return (
        <div className="flex items-center gap-2">
            <span
                className={
                    status === "success"
                        ? "text-green-600"
                        : status === "error"
                          ? "text-red-600"
                          : "text-gray-500"
                }
            >
                {status === "success" && "API通信成功"}
                {status === "error" && "API通信失敗"}
                {status === "loading" && "API通信確認中..."}
            </span>
        </div>
    );
}
