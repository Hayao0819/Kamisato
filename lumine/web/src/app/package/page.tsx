import { Suspense } from "react";
import ClientPackageDetailPage from "./package-client";

export default function PackageDetailPage() {
    return (
        <Suspense>
            <ClientPackageDetailPage />
        </Suspense>
    );
}
