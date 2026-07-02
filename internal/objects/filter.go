package objects

import (
	"context"
	"errors"
	"sort"
	"time"

	"github.com/nats-io/nats.go/jetstream"
)

// FilterRetrievableObjects probes each meta-active object with GetInfo and keeps
// only names that the source bucket can actually serve.
func FilterRetrievableObjects(ctx context.Context, source jetstream.ObjectStore, candidates []*jetstream.ObjectInfo) (migratable []*jetstream.ObjectInfo, omitted []string) {
	for _, info := range candidates {
		if _, err := source.GetInfo(ctx, info.Name); err != nil {
			if errors.Is(err, jetstream.ErrObjectNotFound) {
				omitted = append(omitted, info.Name)
				continue
			}
			// Treat unexpected probe errors as omitted so copy can continue.
			omitted = append(omitted, info.Name)
			continue
		}
		migratable = append(migratable, info)
	}
	sort.Strings(omitted)
	return migratable, omitted
}

// IsLink reports whether the object info describes a link.
func IsLink(info *jetstream.ObjectInfo) bool {
	return info.Opts != nil && info.Opts.Link != nil
}

// CopyTimeout returns a per-object timeout suitable for large blob copies.
func CopyTimeout(requestTimeout time.Duration) time.Duration {
	const minObjectCopyTimeout = 5 * time.Minute
	if requestTimeout > minObjectCopyTimeout {
		return requestTimeout
	}
	return minObjectCopyTimeout
}
