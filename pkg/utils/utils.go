package utils

import (
	"os"
	"regexp"

	"github.com/kardianos/osext"
)

// FileExists checks if a file exists and is not a directory before we
// try using it to prevent further errors.
func FileExists(filename string) bool {
	info, err := os.Stat(filename)
	if os.IsNotExist(err) {
		return false
	}
	return !info.IsDir()
}

// FolderExists checks if a file exists and is not a directory before we
// try using it to prevent further errors.
func FolderExists(filename string) bool {
	info, err := os.Stat(filename)
	if os.IsNotExist(err) {
		return false
	}
	return info.IsDir()
}

// GetCurrentExecPath returns the path of the executable file
func GetCurrentExecPath() string {
	dir, err := osext.ExecutableFolder()
	if err != nil {
		return ""
	}

	return dir
}

// GetParams returns a dictionary of group name and group value in a given regex defined by a string
func GetParams(regEx, str string) (paramsMap map[string]string) {

	var compRegEx = regexp.MustCompile(regEx)
	return GetParamsWithRegex(compRegEx, str)
}

// GetParamsWithRegex returns a dictionary of group name and group value in a given regex
func GetParamsWithRegex(compRegEx *regexp.Regexp, str string) (paramsMap map[string]string) {

	match := compRegEx.FindStringSubmatch(str)

	paramsMap = make(map[string]string)
	for i, name := range compRegEx.SubexpNames() {
		if i > 0 && i <= len(match) {
			paramsMap[name] = match[i]
		}
	}
	return
}
