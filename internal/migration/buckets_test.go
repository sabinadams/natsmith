package migration

import "testing"

type fakeBucketStatus struct {
	name string
}

func (f fakeBucketStatus) Bucket() string { return f.name }

func statusChan(statuses ...fakeBucketStatus) <-chan fakeBucketStatus {
	ch := make(chan fakeBucketStatus, len(statuses))
	for _, status := range statuses {
		ch <- status
	}
	close(ch)
	return ch
}

func TestFilterBucketStatuses(t *testing.T) {
	t.Parallel()

	cfg := BaseConfig{
		Buckets: map[string]struct{}{"keep": {}, "skip": {}},
		Omit:    map[string]struct{}{"skip": {}},
	}

	got := FilterBucketStatuses(cfg, statusChan(
		fakeBucketStatus{name: "keep"},
		fakeBucketStatus{name: "skip"},
		fakeBucketStatus{name: "other"},
	))
	if len(got) != 1 || got[0].Bucket() != "keep" {
		t.Fatalf("filtered buckets: %+v", got)
	}
}

func TestFilterBucketStatusesAllBuckets(t *testing.T) {
	t.Parallel()

	cfg := BaseConfig{Omit: map[string]struct{}{"skip": {}}}

	got := FilterBucketStatuses(cfg, statusChan(
		fakeBucketStatus{name: "a"},
		fakeBucketStatus{name: "skip"},
		fakeBucketStatus{name: "b"},
	))
	if len(got) != 2 {
		t.Fatalf("filtered buckets: %+v", got)
	}
}
