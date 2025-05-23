package raiou

import (
	go_srcinfo "github.com/Morganamilo/go-srcinfo"
)

// RaiouPackage converts a go_srcinfo.Package to a raiou.Package
func RaiouPackage(pkg *go_srcinfo.Package) *Package {
	if pkg == nil {
		return nil
	}

	raiouPkg := &Package{
		PkgName:   pkg.Pkgname,
		PkgDesc:   pkg.Pkgdesc,
		PkgArch:   pkg.Arch,
		URL:       pkg.URL,
		License:   pkg.License,
		Groups:    pkg.Groups,
		Backup:    pkg.Backup,
		Options:   pkg.Options,
		Install:   pkg.Install,
		Changelog: pkg.Changelog,
	}

	// Convert ArchString slices
	raiouPkg.Depends = convertArchStringSlice(pkg.Depends)
	raiouPkg.OptDepends = convertArchStringSlice(pkg.OptDepends)
	raiouPkg.Provides = convertArchStringSlice(pkg.Provides)
	raiouPkg.Conflicts = convertArchStringSlice(pkg.Conflicts)
	raiouPkg.Replaces = convertArchStringSlice(pkg.Replaces)

	return raiouPkg
}

// RaiouPackageBase converts a go_srcinfo.PackageBase to a raiou.PackageBase
func RaiouPackageBase(pkgBase *go_srcinfo.PackageBase) *PackageBase {
	if pkgBase == nil {
		return nil
	}

	raiouPkgBase := &PackageBase{
		PkgBase:      pkgBase.Pkgbase,
		PkgVer:       pkgBase.Pkgver,
		PkgRel:       pkgBase.Pkgrel,
		PkgEpoch:     pkgBase.Epoch,
		ValidPGPKeys: pkgBase.ValidPGPKeys,
		NoExtract:    pkgBase.NoExtract,
	}

	// Convert ArchString slices
	raiouPkgBase.Source = convertArchStringSlice(pkgBase.Source)
	raiouPkgBase.MD5Sums = convertArchStringSlice(pkgBase.MD5Sums)
	raiouPkgBase.SHA1Sums = convertArchStringSlice(pkgBase.SHA1Sums)
	raiouPkgBase.SHA224Sums = convertArchStringSlice(pkgBase.SHA224Sums)
	raiouPkgBase.SHA256Sums = convertArchStringSlice(pkgBase.SHA256Sums)
	raiouPkgBase.SHA384Sums = convertArchStringSlice(pkgBase.SHA384Sums)
	raiouPkgBase.SHA512Sums = convertArchStringSlice(pkgBase.SHA512Sums)
	raiouPkgBase.B2Sums = convertArchStringSlice(pkgBase.B2Sums)
	raiouPkgBase.MakeDepends = convertArchStringSlice(pkgBase.MakeDepends)
	raiouPkgBase.CheckDepends = convertArchStringSlice(pkgBase.CheckDepends)

	return raiouPkgBase
}

// RaiouSRCINFO converts a go_srcinfo.Srcinfo to a raiou.SRCINFO
func RaiouSRCINFO(srcinfo *go_srcinfo.Srcinfo) *SRCINFO {
	if srcinfo == nil {
		return nil
	}

	raiouSRCINFO := &SRCINFO{
		PackageBase: *RaiouPackageBase(&srcinfo.PackageBase),
		Package:     *RaiouPackage(&srcinfo.Package),
		Packages:    make([]Package, len(srcinfo.Packages)),
	}

	for i, pkg := range srcinfo.Packages {
		raiouSRCINFO.Packages[i] = *RaiouPackage(&pkg)
	}

	return raiouSRCINFO
}

// convertArchStringSlice converts a slice of go_srcinfo.ArchString to a map[string]string
func convertArchStringSlice(archStrings []go_srcinfo.ArchString) ArchStrings {
	if archStrings == nil {
		return nil
	}
	result := make(ArchStrings)
	for _, archString := range archStrings {
		result[archString.Arch] = archString.Value
	}

	return result
}

// // archStringSliceToMapHookFunc is a DecodeHookFunc that converts []ArchString to map[string]string
// func archStringSliceToMapHookFunc() mapstructure.DecodeHookFunc {
// 	return func(
// 		f reflect.Type,
// 		t reflect.Type,
// 		data interface{}) (interface{}, error) {
// 		if f.Kind() != reflect.Slice || f.Elem().Kind() != reflect.Struct || f.Elem() != reflect.TypeOf(ArchString{}) {
// 			return data, nil
// 		}
// 		if t.Kind() != reflect.Map || t.Key().Kind() != reflect.String || t.Elem().Kind() != reflect.String {
// 			return data, nil
// 		}

// 		slice, ok := data.([]ArchString)
// 		if !ok {
// 			return data, nil
// 		}

// 		result := make(map[string]string, len(slice))
// 		for _, as := range slice {
// 			result[as.Arch] = as.Value
// 		}
// 		return result, nil
// 	}
// }
