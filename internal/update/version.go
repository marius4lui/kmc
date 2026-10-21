package update

import "strings"

type semanticVersion struct {
	numbers    []string
	prerelease []string
}

// CompareVersions compares SemVer precedence, accepting an optional v prefix
// and surrounding whitespace. Invalid versions (including dev and unversioned
// nightly tags) sort below valid versions. Two invalid versions are compared
// lexically, preserving update checks for dated tags such as nightly-20261021.
func CompareVersions(left, right string) int {
	lv, leftOK := parseVersion(left)
	rv, rightOK := parseVersion(right)
	if !leftOK && !rightOK {
		return strings.Compare(normalizeVersion(left), normalizeVersion(right))
	}
	if !leftOK {
		return -1
	}
	if !rightOK {
		return 1
	}
	for index := range lv.numbers {
		if result := compareNumeric(lv.numbers[index], rv.numbers[index]); result != 0 {
			return result
		}
	}
	if len(lv.prerelease) == 0 && len(rv.prerelease) != 0 {
		return 1
	}
	if len(rv.prerelease) == 0 && len(lv.prerelease) != 0 {
		return -1
	}
	for index := 0; index < len(lv.prerelease) && index < len(rv.prerelease); index++ {
		leftID, rightID := lv.prerelease[index], rv.prerelease[index]
		leftNumeric, rightNumeric := numericIdentifier(leftID), numericIdentifier(rightID)
		result := 0
		switch {
		case leftNumeric && rightNumeric:
			result = compareNumeric(leftID, rightID)
		case leftNumeric:
			result = -1
		case rightNumeric:
			result = 1
		default:
			result = strings.Compare(leftID, rightID)
		}
		if result != 0 {
			return result
		}
	}
	if len(lv.prerelease) < len(rv.prerelease) {
		return -1
	}
	if len(lv.prerelease) > len(rv.prerelease) {
		return 1
	}
	return 0
}

func parseVersion(value string) (semanticVersion, bool) {
	version, build, hasBuild := strings.Cut(normalizeVersion(value), "+")
	if hasBuild && !validIdentifiers(build, false) {
		return semanticVersion{}, false
	}
	core, prerelease, hasPrerelease := strings.Cut(version, "-")
	numbers := strings.Split(core, ".")
	if len(numbers) != 3 {
		return semanticVersion{}, false
	}
	for _, number := range numbers {
		if !numericIdentifier(number) || (len(number) > 1 && number[0] == '0') {
			return semanticVersion{}, false
		}
	}
	parsed := semanticVersion{numbers: numbers}
	if hasPrerelease {
		if !validIdentifiers(prerelease, true) {
			return semanticVersion{}, false
		}
		parsed.prerelease = strings.Split(prerelease, ".")
	}
	return parsed, true
}

func validIdentifiers(value string, prerelease bool) bool {
	for _, identifier := range strings.Split(value, ".") {
		if identifier == "" {
			return false
		}
		for _, char := range identifier {
			if !(char >= '0' && char <= '9' || char >= 'a' && char <= 'z' || char >= 'A' && char <= 'Z' || char == '-') {
				return false
			}
		}
		if prerelease && numericIdentifier(identifier) && len(identifier) > 1 && identifier[0] == '0' {
			return false
		}
	}
	return true
}

func numericIdentifier(value string) bool {
	if value == "" {
		return false
	}
	for _, char := range value {
		if char < '0' || char > '9' {
			return false
		}
	}
	return true
}

// Comparing digit counts before lexical order avoids integer overflow. Both
// inputs are already validated numeric identifiers without leading zeroes.
func compareNumeric(left, right string) int {
	if len(left) < len(right) {
		return -1
	}
	if len(left) > len(right) {
		return 1
	}
	return strings.Compare(left, right)
}
