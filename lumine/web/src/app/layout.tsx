
import type { Metadata } from "next";
import "@/styles/globals.css";
import { Header } from "@/components/header";
import { ToastProvider, ToastViewport } from "@/components/ui/toast";

export const metadata: Metadata = {
    title: "Lumine - Arch Linux パッケージリポジトリ",
    description:
        "LumineはAyatoバックエンドを利用したArch Linux向けの非公式パッケージリポジトリWebフロントエンドです。パッケージの検索・ダウンロードが可能です。",
};

export default function RootLayout({
    children,
}: Readonly<{
    children: React.ReactNode;
}>) {
    return (
        <html lang="en">
            <body>
                <ToastProvider>
                    <Header />
                    {children}
                    {/* トースト通知を画面上部中央に表示 */}
                    <ToastViewport className="fixed top-16 left-1/2 -translate-x-1/2 z-[100] w-full max-w-md flex flex-col items-center" />
                </ToastProvider>
            </body>
        </html>
    );
}
