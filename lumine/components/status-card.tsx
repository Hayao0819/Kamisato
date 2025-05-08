import type { ServerStatus } from "@/lib/data";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Badge } from "@/components/ui/badge";
import { Progress } from "@/components/ui/progress";
import { Clock, HardDrive, Package, RefreshCw } from "lucide-react";

interface StatusCardProps {
    server: ServerStatus;
}

export function StatusCard({ server }: StatusCardProps) {
    const getStatusColor = (status: ServerStatus["status"]) => {
        switch (status) {
            case "online":
                return "bg-green-500";
            case "offline":
                return "bg-red-500";
            case "maintenance":
                return "bg-yellow-500";
            case "syncing":
                return "bg-blue-500";
            default:
                return "bg-gray-500";
        }
    };

    const getStatusText = (status: ServerStatus["status"]) => {
        switch (status) {
            case "online":
                return "オンライン";
            case "offline":
                return "オフライン";
            case "maintenance":
                return "メンテナンス中";
            case "syncing":
                return "同期中";
            default:
                return "不明";
        }
    };

    return (
        <Card className="overflow-hidden">
            <CardHeader className="pb-2">
                <div className="flex justify-between items-center">
                    <CardTitle className="text-base sm:text-lg">
                        {server.name}
                    </CardTitle>
                    <Badge
                        className={`${getStatusColor(server.status)} text-white text-xs`}
                    >
                        {getStatusText(server.status)}
                    </Badge>
                </div>
            </CardHeader>
            <CardContent>
                <div className="grid gap-3 sm:gap-4">
                    <div className="grid grid-cols-1 sm:grid-cols-2 gap-2 sm:gap-4">
                        <div className="flex items-center gap-2">
                            <Clock className="h-3 w-3 sm:h-4 sm:w-4 text-muted-foreground" />
                            <span className="text-xs sm:text-sm">
                                稼働時間: {server.uptime}
                            </span>
                        </div>
                        <div className="flex items-center gap-2">
                            <RefreshCw className="h-3 w-3 sm:h-4 sm:w-4 text-muted-foreground" />
                            <span className="text-xs sm:text-sm">
                                最終同期: {server.lastSync}
                            </span>
                        </div>
                    </div>
                    <div className="grid grid-cols-1 sm:grid-cols-2 gap-2 sm:gap-4">
                        <div className="flex items-center gap-2">
                            <Package className="h-3 w-3 sm:h-4 sm:w-4 text-muted-foreground" />
                            <span className="text-xs sm:text-sm">
                                パッケージ数: {server.packages.toLocaleString()}
                            </span>
                        </div>
                        <div className="flex items-center gap-2">
                            <span className="text-xs sm:text-sm">
                                負荷: {server.load}
                            </span>
                        </div>
                    </div>
                    <div>
                        <div className="flex items-center justify-between mb-1">
                            <HardDrive className="h-3 w-3 sm:h-4 sm:w-4 text-muted-foreground" />
                            <span className="text-xs sm:text-sm">
                                ディスク使用量: {server.diskUsage}%
                            </span>
                        </div>
                        <Progress
                            value={server.diskUsage}
                            className="h-1.5 sm:h-2"
                        />
                    </div>
                </div>
            </CardContent>
        </Card>
    );
}
