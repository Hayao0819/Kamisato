import { Suspense } from "react";
import { BuildsPageClient } from "./builds-client";

export default function BuildsPage() {
    return (
        <Suspense>
            <BuildsPageClient />
        </Suspense>
    );
}
