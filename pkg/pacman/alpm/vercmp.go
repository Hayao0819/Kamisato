package alpm

import "strings"

// VerCmp compares two pacman version strings ([epoch:]version[-pkgrel]); returns -1, 0, or 1.
// Pure-Go port of libalpm's alpm_pkg_vercmp so the depsolver avoids forking vercmp;
// error is always nil (kept for API compatibility).
func VerCmp(v1, v2 string) (int, error) {
	if v1 == v2 {
		return 0, nil
	}

	e1, ver1, rel1, hasRel1 := parseEVR(v1)
	e2, ver2, rel2, hasRel2 := parseEVR(v2)

	ret := rpmvercmp(e1, e2)
	if ret == 0 {
		ret = rpmvercmp(ver1, ver2)
		// pkgrel is only significant when both versions carry one.
		if ret == 0 && hasRel1 && hasRel2 {
			ret = rpmvercmp(rel1, rel2)
		}
	}
	return ret, nil
}

// parseEVR splits [epoch:]version[-pkgrel] into its components. A missing epoch
// defaults to "0"; hasRel reports whether a pkgrel was present.
func parseEVR(evr string) (epoch, version, release string, hasRel bool) {
	i := 0
	for i < len(evr) && isDigit(evr[i]) {
		i++
	}
	if i < len(evr) && evr[i] == ':' {
		epoch = evr[:i]
		if epoch == "" {
			epoch = "0"
		}
		version = evr[i+1:]
	} else {
		epoch = "0"
		version = evr
	}

	if j := strings.LastIndexByte(version, '-'); j >= 0 {
		release = version[j+1:]
		version = version[:j]
		hasRel = true
	}
	return epoch, version, release, hasRel
}

// rpmvercmp compares two version (or epoch/pkgrel) strings segment by segment,
// following the rpmvercmp semantics used by libalpm.
func rpmvercmp(a, b string) int {
	if a == b {
		return 0
	}

	one, two := 0, 0
	ptr1, ptr2 := 0, 0

	for one < len(a) && two < len(b) {
		for one < len(a) && !isAlnum(a[one]) {
			one++
		}
		for two < len(b) && !isAlnum(b[two]) {
			two++
		}
		if one >= len(a) || two >= len(b) {
			break
		}

		// Differing separator lengths decide the comparison on their own.
		if (one - ptr1) != (two - ptr2) {
			if (one - ptr1) < (two - ptr2) {
				return -1
			}
			return 1
		}

		ptr1, ptr2 = one, two

		isnum := isDigit(a[ptr1])
		if isnum {
			for ptr1 < len(a) && isDigit(a[ptr1]) {
				ptr1++
			}
			for ptr2 < len(b) && isDigit(b[ptr2]) {
				ptr2++
			}
		} else {
			for ptr1 < len(a) && isAlpha(a[ptr1]) {
				ptr1++
			}
			for ptr2 < len(b) && isAlpha(b[ptr2]) {
				ptr2++
			}
		}

		seg1, seg2 := a[one:ptr1], b[two:ptr2]

		// One side has a segment of the other type (the alpha walk produced an
		// empty run against a numeric one): numeric always outranks alpha.
		if len(seg2) == 0 {
			if isnum {
				return 1
			}
			return -1
		}

		if isnum {
			seg1 = strings.TrimLeft(seg1, "0")
			seg2 = strings.TrimLeft(seg2, "0")
			// With leading zeros gone, the longer digit run is the larger number.
			if len(seg1) > len(seg2) {
				return 1
			}
			if len(seg2) > len(seg1) {
				return -1
			}
		}

		if rc := strings.Compare(seg1, seg2); rc != 0 {
			return rc
		}

		one, two = ptr1, ptr2
	}

	// Both exhausted: every segment matched and only separators differed.
	if one >= len(a) && two >= len(b) {
		return 0
	}

	// A leftover alpha segment loses to an empty string; a leftover numeric wins.
	oneEmpty := one >= len(a)
	twoAlpha := two < len(b) && isAlpha(b[two])
	oneAlpha := one < len(a) && isAlpha(a[one])
	if (oneEmpty && !twoAlpha) || oneAlpha {
		return -1
	}
	return 1
}

func isDigit(c byte) bool { return c >= '0' && c <= '9' }

func isAlpha(c byte) bool {
	return (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z')
}

func isAlnum(c byte) bool { return isDigit(c) || isAlpha(c) }
