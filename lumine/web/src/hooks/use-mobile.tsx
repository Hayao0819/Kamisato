"use client";

import { useEffect, useState } from "react";

export function useMobile() {
    const [isMobile, setIsMobile] = useState(false);

    useEffect(() => {
        // 初期状態を設定
        setIsMobile(window.innerWidth < 768);

        // リサイズイベントのリスナーを追加
        const handleResize = () => {
            setIsMobile(window.innerWidth < 768);
        };

        window.addEventListener("resize", handleResize);

        // クリーンアップ関数
        return () => {
            window.removeEventListener("resize", handleResize);
        };
    }, []);

    return isMobile;
}
