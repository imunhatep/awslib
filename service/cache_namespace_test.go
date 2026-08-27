package service_test

import (
	"os"
	"path/filepath"
	"regexp"
	"testing"
)

// namespaceExpr matches the namespace line the generate-cached template emits, capturing
// the format string so the service suffix baked into it can be compared across packages.
var namespaceExpr = regexp.MustCompile(`ns := fmt\.Sprintf\("([^"]+)", r\.client\.GetAccountID\(\), r\.client\.GetRegion\(\)\)`)

// TestCachedRepositoryNamespacesAreUnique guards the cache key space against collisions
// between services.
//
// cache.DataCache builds its key as "<namespace>:<name>", and cache.Key renders a call
// with no parameters as the bare method name. Several services expose identically named
// methods — ListClustersAll on ecs/eks/emr, ListTablesAll on dynamodb/glue — so a
// namespace of only "<accountID>:<region>" makes those repositories share one key. They
// then overwrite each other's entries and every read decodes another service's payload,
// which surfaces at runtime as "gob: wrong type ... for received field Cluster.Tags" and
// a silent refetch from AWS on every cycle.
//
// The namespace therefore has to carry the service. Reading it back out of the generated
// files keeps the template honest and catches a hand-edit that drops the suffix.
func TestCachedRepositoryNamespacesAreUnique(t *testing.T) {
	files, err := filepath.Glob(filepath.Join("*", "cached_repository.go"))
	if err != nil {
		t.Fatalf("glob cached repositories: %v", err)
	}

	if len(files) == 0 {
		t.Fatal("no generated cached_repository.go files found")
	}

	namespaces := make(map[string]string, len(files))
	for _, file := range files {
		src, err := os.ReadFile(file)
		if err != nil {
			t.Fatalf("read %s: %v", file, err)
		}

		match := namespaceExpr.FindSubmatch(src)
		if match == nil {
			t.Errorf("%s: no cache namespace assignment found", file)
			continue
		}

		namespace := string(match[1])
		if previous, seen := namespaces[namespace]; seen {
			t.Errorf(
				"%s and %s share cache namespace %q; identically named methods in the two "+
					"packages would overwrite each other's cache entries",
				previous, file, namespace,
			)

			continue
		}

		namespaces[namespace] = file
	}
}
