package builder

import (
	"reflect"
	"testing"
)

func TestSpecMakepkgArgs(t *testing.T) {
	runCheck := false
	runVerify := false
	spec := Spec{
		IgnoreArch:    true,
		RunCheck:      &runCheck,
		RunVerify:     &runVerify,
		SkipChecksums: true,
		SkipPGPCheck:  true,
	}
	want := []string{"--ignorearch", "--nocheck", "--noverify", "--skipchecksums", "--skippgpcheck"}
	if got := spec.MakepkgArgs(); !reflect.DeepEqual(got, want) {
		t.Fatalf("MakepkgArgs() = %v, want %v", got, want)
	}
}
