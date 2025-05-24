import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Badge } from "@/components/ui/badge";

interface ServerStatus {
    id: string;
    name: string;
    status: string;
}

interface StatusCardProps {
    server: ServerStatus;
}

export function StatusCard({ server }: StatusCardProps) {
    const getStatusColor = (status: string) => {
        switch (status) {
            case "Online":
                return "bg-green-500";
            case "Offline":
                return "bg-red-500";
            default:
                return "bg-gray-500";
        }
    };

    const getStatusText = (status: string) => {
        switch (status) {
            case "Online":
                return "オンライン";
            case "Offline":
                return "オフライン";
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
                <div className="text-sm text-muted-foreground">
                    ステータス: {getStatusText(server.status)}
                </div>
            </CardContent>
        </Card>
    );
}
