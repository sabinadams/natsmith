package migrate

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
)

const (
	kvStreamPrefix      = "KV_"
	kvSubjectFilterTmpl = "$KV.%s.>"
	kvSubjectPrefixTmpl = "$KV.%s."
	kvOpHeader          = "KV-Operation"
)

// KVBucketSnapshot is a point-in-time view of a KV bucket from the backing stream.
type KVBucketSnapshot struct {
	Listed       []string
	Migratable   []string
	Omitted      []string
	Values       map[string][]byte
	MessageCount int
}

// StreamScanProgress reports progress while scanning a KV backing stream.
type StreamScanProgress struct {
	StreamMessages int
	Scanned        int
	UniqueKeys     int
}

// StreamScanReporter receives periodic updates during a stream scan.
type StreamScanReporter func(StreamScanProgress)

// SnapshotKVFromStream scans the backing JetStream and derives the latest state
// per key from stream messages. This is deterministic for a fixed stream state.
func SnapshotKVFromStream(ctx context.Context, js jetstream.JetStream, bucket string, report StreamScanReporter) (KVBucketSnapshot, error) {
	stream, err := js.Stream(ctx, kvStreamPrefix+bucket)
	if err != nil {
		return KVBucketSnapshot{}, fmt.Errorf("open stream %s: %w", kvStreamPrefix+bucket, err)
	}

	info, err := stream.Info(ctx)
	if err != nil {
		return KVBucketSnapshot{}, fmt.Errorf("stream info: %w", err)
	}

	subjectPrefix := fmt.Sprintf(kvSubjectPrefixTmpl, bucket)
	filter := fmt.Sprintf(kvSubjectFilterTmpl, bucket)
	totalMessages := int(info.State.Msgs)

	reportScan := func(scanned, keys int) {
		if report == nil {
			return
		}
		report(StreamScanProgress{
			StreamMessages: totalMessages,
			Scanned:        scanned,
			UniqueKeys:     keys,
		})
	}
	reportScan(0, 0)

	if totalMessages == 0 {
		return KVBucketSnapshot{Values: make(map[string][]byte)}, nil
	}

	cons, err := stream.OrderedConsumer(ctx, jetstream.OrderedConsumerConfig{
		FilterSubjects: []string{filter},
	})
	if err != nil {
		return KVBucketSnapshot{}, fmt.Errorf("create ordered consumer: %w", err)
	}

	msgs, err := cons.Messages(jetstream.StopAfter(totalMessages))
	if err != nil {
		return KVBucketSnapshot{}, fmt.Errorf("consume stream messages: %w", err)
	}
	defer msgs.Stop()

	type keyState struct {
		seq   uint64
		op    jetstream.KeyValueOp
		value []byte
	}

	states := make(map[string]keyState)
	messageCount := 0
	lastReport := time.Now()

	for {
		msg, err := msgs.Next()
		if err != nil {
			if errors.Is(err, jetstream.ErrMsgIteratorClosed) {
				break
			}
			return KVBucketSnapshot{}, fmt.Errorf("read stream message: %w", err)
		}

		meta, err := msg.Metadata()
		if err != nil {
			return KVBucketSnapshot{}, fmt.Errorf("message metadata: %w", err)
		}

		key, ok := keyFromSubject(msg.Subject(), subjectPrefix)
		if !ok {
			continue
		}

		next := keyState{
			seq:   meta.Sequence.Stream,
			op:    kvOpFromHeaders(msg.Headers()),
			value: append([]byte(nil), msg.Data()...),
		}
		if prev, exists := states[key]; !exists || next.seq >= prev.seq {
			states[key] = next
		}
		messageCount++
		if report != nil && (messageCount == totalMessages || messageCount%250 == 0 || time.Since(lastReport) >= 2*time.Second) {
			reportScan(messageCount, len(states))
			lastReport = time.Now()
		}
	}

	reportScan(messageCount, len(states))

	snap := KVBucketSnapshot{
		Values:       make(map[string][]byte),
		MessageCount: messageCount,
	}

	for key, state := range states {
		snap.Listed = append(snap.Listed, key)
		switch state.op {
		case jetstream.KeyValuePut:
			snap.Migratable = append(snap.Migratable, key)
			snap.Values[key] = state.value
		default:
			snap.Omitted = append(snap.Omitted, key)
		}
	}

	sort.Strings(snap.Listed)
	sort.Strings(snap.Migratable)
	sort.Strings(snap.Omitted)

	if info.State.Msgs > 0 && messageCount == 0 {
		return KVBucketSnapshot{}, fmt.Errorf("stream has %d messages but scan returned none", info.State.Msgs)
	}

	return snap, nil
}

func keyFromSubject(subject, prefix string) (string, bool) {
	if !strings.HasPrefix(subject, prefix) {
		return "", false
	}
	key := strings.TrimPrefix(subject, prefix)
	return key, key != ""
}

func kvOpFromHeaders(h nats.Header) jetstream.KeyValueOp {
	if h == nil {
		return jetstream.KeyValuePut
	}
	switch h.Get(kvOpHeader) {
	case "DEL":
		return jetstream.KeyValueDelete
	case "PURGE":
		return jetstream.KeyValuePurge
	}
	switch h.Get(jetstream.MarkerReasonHeader) {
	case "MaxAge", "Purge":
		return jetstream.KeyValuePurge
	case "Remove":
		return jetstream.KeyValueDelete
	}
	return jetstream.KeyValuePut
}
