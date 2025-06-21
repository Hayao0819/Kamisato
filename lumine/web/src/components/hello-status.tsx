"use client";

import { getHelloEndpoint } from "@/lib/api";
import { useEffect, useState } from "react";

export function HelloStatus() {
    const [status, setStatus] = useState<"success" | "error" | "loading">(
        "loading",
    );

    useEffect(() => {
        const checkHello = async () => {
            setStatus("loading");
            try {
                const res = await fetch(getHelloEndpoint());
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
    }, []);

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
