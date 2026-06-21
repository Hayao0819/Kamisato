import { Suspense } from "react";
import PackagesClient from "./packages-client";

// パッケージ (Packages) category. The console table view, driven entirely by the
// URL query (?repo=&arch=&q=&group=&type=&sort=&dir=&page=&per=). The client
// reads search params, so it must sit under a <Suspense> boundary for the
// static export build.
export default function PackagesPage() {
    return (
        <Suspense>
            <PackagesClient />
        </Suspense>
    );
}
