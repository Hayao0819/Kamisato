import type { Metadata } from "next";
import "@/styles/globals.css";
import { Footer } from "@/components/footer";
import { Header } from "@/components/header";
import LumineProvider from "@/components/lumine-provider";
import { AuthProvider } from "@/components/auth-provider";
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
        <html lang="ja">
            <head>
                <meta charSet="UTF-8" />
                <meta
                    name="viewport"
                    content="width=device-width, initial-scale=1"
                />
            </head>
            <ToastProvider>
                <AuthProvider>
                    <LumineProvider>
                        <body className="h-screen flex flex-col">
                            <Header />
                            <main className="grow overflow-scroll hidden-scrollbar">
                                {children}
                            </main>
                            <Footer />
                            <ToastViewport className="fixed top-16 left-1/2 -translate-x-1/2 z-[100] w-full max-w-md flex flex-col items-center" />
                        </body>
                    </LumineProvider>
                </AuthProvider>
            </ToastProvider>
        </html>
    );
}
