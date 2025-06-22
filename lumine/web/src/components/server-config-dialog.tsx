"use client";

import { useEffect, useState } from "react";
import {
    Dialog,
    DialogContent,
    DialogDescription,
    DialogFooter,
    DialogHeader,
    DialogTitle,
    DialogTrigger,
} from "@/components/ui/dialog";
import { Input } from "@/components/ui/input";
import { Button } from "@/components/ui/button";
import { Label } from "@/components/ui/label";
import { useToast } from "@/hooks/use-toast";
import { Settings } from "lucide-react";

const STORAGE_KEY = "lumine_api_base_url";

export function ServerConfigDialog() {
    const [open, setOpen] = useState(false);
    const [url, setUrl] = useState("");
    const { toast } = useToast();

    useEffect(() => {
        const saved = localStorage.getItem(STORAGE_KEY);
        if (saved) setUrl(saved);
    }, [open]);

    const handleSave = () => {
        localStorage.setItem(STORAGE_KEY, url);
        toast({ title: "サーバーURLを保存しました" });
        setOpen(false);
        window.location.reload(); // 設定反映のためリロード
    };

    return (
        <Dialog open={open} onOpenChange={setOpen}>
            <DialogTrigger asChild>
                <Button variant="ghost" size="icon" aria-label="サーバー設定">
                    <Settings className="h-5 w-5" />
                </Button>
            </DialogTrigger>
            <DialogContent>
                <DialogHeader>
                    <DialogTitle>APIサーバー設定</DialogTitle>
                    <DialogDescription>
                        リクエストを送信するAPIサーバーのURLを入力してください。
                    </DialogDescription>
                </DialogHeader>
                <div className="space-y-2">
                    <Label htmlFor="server-url">サーバーURL</Label>
                    <Input
                        id="server-url"
                        value={url}
                        onChange={e => setUrl(e.target.value)}
                        placeholder="http://localhost:9000"
                    />
                </div>
                <DialogFooter>
                    <Button onClick={handleSave}>保存</Button>
                </DialogFooter>
            </DialogContent>
        </Dialog>
    );
}
