package configcheck

import (
	"os"
	"path/filepath"
	"runtime"
	"sigs.k8s.io/yaml"
	"strconv"
	"strings"
	"testing"
)

func TestWorkbench(t *testing.T) {
	_, callerFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	workbenchPath := filepath.Join(filepath.Dir(callerFile), "..", "..", "workbench.yaml")
	data, err := os.ReadFile(workbenchPath)
	if err != nil {
		t.Fatalf("read workbench.yaml: %v", err)
	}

	var document map[string]any
	if err := yaml.Unmarshal(data, &document); err != nil {
		t.Fatalf("parse workbench.yaml: %v", err)
	}

	if got, ok := document["team"].(string); !ok || got != "engineering" {
		t.Fatalf("team must be the generic engineering metadata value, got %#v", document["team"])
	}
	if got, ok := document["APP_TITLE"].(string); !ok || got != "Go App Utils" {
		t.Fatalf("APP_TITLE must be Go App Utils, got %#v", document["APP_TITLE"])
	}
	patterns, ok := document["ALLOW_PR_TO_MASTER_FROM"].([]any)
	if !ok || len(patterns) != 3 {
		t.Fatalf("ALLOW_PR_TO_MASTER_FROM must be a three-entry list, got %#v", document["ALLOW_PR_TO_MASTER_FROM"])
	}
	for i, pattern := range patterns {
		if _, ok := pattern.(string); !ok {
			t.Fatalf("ALLOW_PR_TO_MASTER_FROM[%d] must be a string, got %#v", i, pattern)
		}
	}

	forbidden := map[string]struct{}{
		"confluence": {}, "publish": {}, "publication": {}, "semantic-release": {},
		"version": {}, "tag": {}, "artifactory": {}, "registry": {}, "email": {},
	}
	checkWorkbenchValues(t, document, "$", forbidden)
}

func checkWorkbenchValues(t *testing.T, value any, path string, forbidden map[string]struct{}) {
	t.Helper()
	switch value := value.(type) {
	case map[string]any:
		for key, child := range value {
			if _, found := forbidden[strings.ToLower(key)]; found {
				t.Errorf("forbidden workbench key %q at %s", key, path)
			}
			checkWorkbenchValues(t, child, path+"."+key, forbidden)
		}
	case []any:
		for i, child := range value {
			checkWorkbenchValues(t, child, path+"["+strconv.Itoa(i)+"]", forbidden)
		}
	case string:
		if _, found := forbidden[strings.ToLower(value)]; found {
			t.Errorf("forbidden workbench value %q at %s", value, path)
		}
	}
}
