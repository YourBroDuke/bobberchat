package protocol

import (
	"fmt"
	"strconv"
	"strings"
)

const ProtocolVersion = "1.0.0"

func IsCompatibleVersion(version string) bool {
	cmp, err := CompareVersions(version, ProtocolVersion)
	if err != nil {
		return false
	}

	requestedMajor, _, _, err := parseVersion(version)
	if err != nil {
		return false
	}

	currentMajor, _, _, err := parseVersion(ProtocolVersion)
	if err != nil {
		return false
	}

	if requestedMajor != currentMajor {
		return false
	}

	return cmp <= 0
}

func CompareVersions(a, b string) (int, error) {
	ma, mia, pa, err := parseVersion(a)
	if err != nil {
		return 0, fmt.Errorf("invalid version %q: %w", a, err)
	}
	mb, mib, pb, err := parseVersion(b)
	if err != nil {
		return 0, fmt.Errorf("invalid version %q: %w", b, err)
	}

	if ma != mb {
		if ma < mb {
			return -1, nil
		}
		return 1, nil
	}

	if mia != mib {
		if mia < mib {
			return -1, nil
		}
		return 1, nil
	}

	if pa != pb {
		if pa < pb {
			return -1, nil
		}
		return 1, nil
	}

	return 0, nil
}

func parseVersion(v string) (major int, minor int, patch int, err error) {
	parts := strings.Split(v, ".")
	if len(parts) != 3 {
		return 0, 0, 0, fmt.Errorf("must be semver core format major.minor.patch")
	}

	major, err = strconv.Atoi(parts[0])
	if err != nil {
		return 0, 0, 0, err
	}
	minor, err = strconv.Atoi(parts[1])
	if err != nil {
		return 0, 0, 0, err
	}
	patch, err = strconv.Atoi(parts[2])
	if err != nil {
		return 0, 0, 0, err
	}

	if major < 0 || minor < 0 || patch < 0 {
		return 0, 0, 0, fmt.Errorf("version numbers must be non-negative")
	}

	return major, minor, patch, nil
}
