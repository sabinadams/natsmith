package migration

type bucketNamed interface {
	Bucket() string
}

// FilterBucketStatuses returns statuses whose bucket name passes cfg filters.
func FilterBucketStatuses[T bucketNamed](cfg BaseConfig, statuses <-chan T) []T {
	var buckets []T
	for status := range statuses {
		if cfg.ShouldMigrateBucket(status.Bucket()) {
			buckets = append(buckets, status)
		}
	}
	return buckets
}
