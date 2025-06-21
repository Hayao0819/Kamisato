import { RefreshButton } from "@/components/refresh-button";
import { StatusCard } from "@/components/status-card";
import { Button } from "@/components/ui/button";
import { getHelloEndpoint, getTeapotEndpoint } from "@/lib/api";
import { ArrowLeft } from "lucide-react";
import Link from "next/link";

async function checkServerStatus() {
    const helloStatus = await fetch(getHelloEndpoint())
        .then((res) => (res.ok ? "Online" : "Offline"))
        .catch(() => "Offline");
    const teapotStatus = await fetch(getTeapotEndpoint())
        .then((res) => (res.ok ? "Online" : "Offline"))
        .catch(() => "Offline");

    return [
        { id: "hello", name: "Hello Endpoint", status: helloStatus },
        { id: "teapot", name: "Teapot Endpoint", status: teapotStatus },
        // Add other relevant endpoints to check as needed
    ];
}

export default async function ServerStatus() {
    const servers = await checkServerStatus();

    return (
        <div className="container mx-auto py-4 sm:py-8 px-4 sm:px-6">
            <header className="mb-6 sm:mb-8">
                <div className="flex flex-col sm:flex-row justify-between items-start sm:items-center gap-4 mb-4">
                    <h1 className="text-2xl sm:text-3xl font-bold">
                        サーバーステータス
                    </h1>
                    <div className="flex gap-2 w-full sm:w-auto">
                        <RefreshButton />
                        <Link href="/" className="flex-1 sm:flex-auto">
                            <Button variant="outline" className="w-full">
                                <ArrowLeft className="h-4 w-4 mr-2" />
                                戻る
                            </Button>
                        </Link>
                    </div>
                </div>
                <p className="text-sm sm:text-base text-muted-foreground">
                    Ayaka バックエンドの各エンドポイントの状態を確認できます。
                </p>
            </header>

            <main>
                <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-4 sm:gap-6">
                    {servers.map((server) => (
                        <StatusCard key={server.id} server={server} />
                    ))}
                </div>
            </main>

            <footer className="mt-8 sm:mt-12 text-center text-xs sm:text-sm text-muted-foreground py-4">
                <p>© 2023 山田ハヤオ / Lumine</p>
            </footer>
        </div>
    );
}
