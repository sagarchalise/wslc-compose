// Package naming resolves project/resource names the same way docker-compose does:
// -p flag > name: key in the compose file > sanitized directory basename.
package naming

import (
	"path/filepath"
	"regexp"
	"strings"
)

var invalidChars = regexp.MustCompile(`[^a-z0-9_-]+`)

func Sanitize(name string) string {
	s := strings.ToLower(name)
	s = invalidChars.ReplaceAllString(s, "-")
	s = strings.Trim(s, "-_")
	if s == "" {
		s = "wslc-compose"
	}
	return s
}

func ResolveProjectName(flagValue, fileNameKey, composeDir string) string {
	if flagValue != "" {
		return Sanitize(flagValue)
	}
	if fileNameKey != "" {
		return Sanitize(fileNameKey)
	}
	return Sanitize(filepath.Base(filepath.Clean(composeDir)))
}

func Service(project, service string) string {
	return project + "-" + service
}

func Network(project, key string) string {
	return project + "-" + key
}

func Volume(project, key string) string {
	return project + "-" + key
}
