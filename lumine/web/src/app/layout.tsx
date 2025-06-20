import type { Metadata } from "next";
import "@/styles/globals.css";

export const metadata: Metadata = {
    title: "Lumine - Arch Linux パッケージリポジトリ",
    description: "LumineはAyatoバックエンドを利用したArch Linux向けの非公式パッケージリポジトリWebフロントエンドです。パッケージの検索・ダウンロードが可能です。",
};

export default function RootLayout({
    children,
}: Readonly<{
    children: React.ReactNode;
}>) {
    return (
        <html lang="en">
            <body>{children}</body>
        </html>
    );
}
