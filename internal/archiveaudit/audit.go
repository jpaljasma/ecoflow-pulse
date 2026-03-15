package archiveaudit

import (
	"fmt"
	"sort"
	"strings"

	"github.com/jpaljasma/ecoflow-pulse/internal/replaycli"
)

type Report struct {
	ManifestObjects        int
	DirectObjects          int
	MissingInArchiveCount  int
	MissingInManifestCount int
	MissingInArchiveKeys   []string
	MissingInManifestKeys  []string
}

func Compare(manifestObjects, directObjects []replaycli.ManifestObject) Report {
	manifestMap := objectKeySet(manifestObjects)
	directMap := objectKeySet(directObjects)

	report := Report{
		ManifestObjects: len(manifestMap),
		DirectObjects:   len(directMap),
	}
	for key := range manifestMap {
		if _, ok := directMap[key]; !ok {
			report.MissingInArchiveKeys = append(report.MissingInArchiveKeys, key)
		}
	}
	for key := range directMap {
		if _, ok := manifestMap[key]; !ok {
			report.MissingInManifestKeys = append(report.MissingInManifestKeys, key)
		}
	}
	sort.Strings(report.MissingInArchiveKeys)
	sort.Strings(report.MissingInManifestKeys)
	report.MissingInArchiveCount = len(report.MissingInArchiveKeys)
	report.MissingInManifestCount = len(report.MissingInManifestKeys)
	return report
}

func (r Report) Healthy() bool {
	return r.MissingInArchiveCount == 0 && r.MissingInManifestCount == 0
}

func ObjectsFromCompositeKeys(keys []string) ([]replaycli.ManifestObject, error) {
	if len(keys) == 0 {
		return nil, nil
	}
	seen := make(map[string]struct{}, len(keys))
	objects := make([]replaycli.ManifestObject, 0, len(keys))
	for _, raw := range keys {
		bucket, key, err := splitCompositeObjectKey(raw)
		if err != nil {
			return nil, err
		}
		composite := bucket + "|" + key
		if _, exists := seen[composite]; exists {
			continue
		}
		seen[composite] = struct{}{}
		objects = append(objects, replaycli.ManifestObject{
			ObjectBucket: bucket,
			ObjectKey:    key,
		})
	}
	return objects, nil
}

func objectKeySet(objects []replaycli.ManifestObject) map[string]struct{} {
	out := make(map[string]struct{}, len(objects))
	for _, object := range objects {
		bucket := strings.TrimSpace(object.ObjectBucket)
		key := strings.Trim(strings.TrimSpace(object.ObjectKey), "/")
		if bucket == "" || key == "" {
			continue
		}
		out[bucket+"|"+key] = struct{}{}
	}
	return out
}

func splitCompositeObjectKey(raw string) (string, string, error) {
	composite := strings.TrimSpace(raw)
	bucket, key, ok := strings.Cut(composite, "|")
	bucket = strings.TrimSpace(bucket)
	key = strings.Trim(strings.TrimSpace(key), "/")
	if !ok || bucket == "" || key == "" {
		return "", "", fmt.Errorf("invalid composite object key %q", raw)
	}
	return bucket, key, nil
}
