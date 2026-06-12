package git

import (
	"regexp"
	"strings"
)

var (
	multiHyphen      = regexp.MustCompile(`-{2,}`)
	consecutiveDots  = regexp.MustCompile(`\.\.`)
	invalidRefChars  = regexp.MustCompile(`[^a-z0-9._/-]`)
)

func SanitizeBranchName(name string) string {
	name = strings.ToLower(name)
	name = strings.ReplaceAll(name, " ", "-")
	name = invalidRefChars.ReplaceAllString(name, "-")
	name = multiHyphen.ReplaceAllString(name, "-")
	name = consecutiveDots.ReplaceAllString(name, ".")
	name = strings.ReplaceAll(name, "@{", "-")
	name = strings.Trim(name, "-._/")
	name = strings.TrimSuffix(name, ".lock")

	if len(name) > 250 {
		name = name[:250]
	}

	if name == "" {
		return "feature"
	}

	return name
}
