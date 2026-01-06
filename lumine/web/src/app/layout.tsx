import type { Metadata } from "next";
import "@/styles/globals.css";
import { AuthProvider } from "@/components/auth-provider";
import { Footer } from "@/components/footer";
import { Header } from "@/components/header";
import LumineProvider from "@/components/lumine-provider";
import { ThemeProvider } from "@/components/theme-provider";
import { ToastProvider, ToastViewport } from "@/components/ui/toast";
import { Toaster } from "@/components/ui/toaster";

export const metadata: Metadata = {
    title: "Arch Linux パッケージリポジトリ - Lumine",
    description:
        "Ayakaバックエンドを利用したArch Linux向けの非公式パッケージリポジトリWebフロントエンド。",
};

export default function RootLayout({
    children,
}: Readonly<{
    children: React.ReactNode;
}>) {
    return (
        <html lang="ja" suppressHydrationWarning>
            <head>
                <meta charSet="UTF-8" />
                <meta
                    name="viewport"
                    content="width=device-width, initial-scale=1"
                />
            </head>
            <body className="min-h-screen flex flex-col bg-background text-foreground antialiased">
                <ThemeProvider
                    attribute="class"
                    defaultTheme="system"
                    enableSystem
                    disableTransitionOnChange
                >
                    <ToastProvider>
                        <AuthProvider>
                            <LumineProvider>
                                <Header />
                                <main className="flex-1">{children}</main>
                                <Footer />
                                <Toaster />
                                <ToastViewport className="fixed top-20 left-1/2 -translate-x-1/2 z-100 w-full max-w-md flex flex-col items-center" />
                            </LumineProvider>
                        </AuthProvider>
                    </ToastProvider>
                </ThemeProvider>
            </body>
        </html>
    );
}
