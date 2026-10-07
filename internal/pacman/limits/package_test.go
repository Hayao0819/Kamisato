package limits

import "testing"

func TestPackageBytes(t *testing.T) {
	if got := PackageBytes(0); got != DefaultPackageBytes {
		t.Fatalf("default = %d, want %d", got, DefaultPackageBytes)
	}
	if got := PackageBytes(123); got != 123 {
		t.Fatalf("configured = %d, want 123", got)
	}
	if got := MultipartBytes(123); got != 123+MaxSignatureBytes+MultipartOverheadBytes {
		t.Fatalf("multipart = %d", got)
	}
	if got := BatchPackages(0); got != DefaultBatchPackages {
		t.Fatalf("batch packages = %d", got)
	}
	if got := BatchBytes(0, 123); got != DefaultBatchBytes {
		t.Fatalf("batch bytes = %d", got)
	}
}
