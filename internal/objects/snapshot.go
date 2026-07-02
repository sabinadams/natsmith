package objects

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/nats-io/nats.go/jetstream"
	"github.com/sabinadams/natsmith/internal/progress"
)

const (
	streamPrefix    = "OBJ_"
	metaSubjectTmpl = "$O.%s.M."
)

// BucketSnapshot is a point-in-time view of an object store from its meta stream.
type BucketSnapshot struct {
	Listed       []string
	Migratable   []*jetstream.ObjectInfo
	Omitted      []string
	MessageCount int
}

// SnapshotFromStream scans OBJ_<bucket> meta messages and derives the latest
// state per object from meta subjects. This is deterministic for a fixed stream state.
func SnapshotFromStream(ctx context.Context, js jetstream.JetStream, bucket string, report func(progress.ScanProgress)) (BucketSnapshot, error) {
	stream, err := js.Stream(ctx, streamPrefix+bucket)
	if err != nil {
		return BucketSnapshot{}, fmt.Errorf("open stream %s: %w", streamPrefix+bucket, err)
	}

	info, err := stream.Info(ctx)
	if err != nil {
		return BucketSnapshot{}, fmt.Errorf("stream info: %w", err)
	}

	metaPrefix := fmt.Sprintf(metaSubjectTmpl, bucket)
	totalMessages := int(info.State.Msgs)

	reportScan := func(scanned, objects int) {
		if report == nil {
			return
		}
		report(progress.ScanProgress{
			StreamMessages: totalMessages,
			Scanned:        scanned,
			Unique:         objects,
			UniqueLabel:    "objects",
		})
	}
	reportScan(0, 0)

	if totalMessages == 0 {
		return BucketSnapshot{}, nil
	}

	// Do not filter the consumer: OBJ_<bucket> holds meta ($O.<bucket>.M.*) and
	// chunk ($O.<bucket>.C.*) messages. StopAfter must match the full stream
	// message count; a meta-only filter would hang waiting for chunk slots.
	cons, err := stream.OrderedConsumer(ctx, jetstream.OrderedConsumerConfig{})
	if err != nil {
		return BucketSnapshot{}, fmt.Errorf("create ordered consumer: %w", err)
	}

	msgs, err := cons.Messages(jetstream.StopAfter(totalMessages))
	if err != nil {
		return BucketSnapshot{}, fmt.Errorf("consume stream messages: %w", err)
	}
	defer msgs.Stop()

	type objectState struct {
		seq  uint64
		info jetstream.ObjectInfo
	}

	// Key by meta subject — one subject per object name (matches GetInfo lookup).
	states := make(map[string]objectState)
	scanned := 0
	metaCount := 0
	lastReport := time.Now()

	for {
		msg, err := msgs.Next()
		if err != nil {
			if errors.Is(err, jetstream.ErrMsgIteratorClosed) {
				break
			}
			return BucketSnapshot{}, fmt.Errorf("read stream message: %w", err)
		}
		scanned++

		subject := msg.Subject()
		if !strings.HasPrefix(subject, metaPrefix) {
			if report != nil && (scanned == totalMessages || scanned%250 == 0 || time.Since(lastReport) >= 2*time.Second) {
				reportScan(scanned, len(states))
				lastReport = time.Now()
			}
			continue
		}

		meta, err := msg.Metadata()
		if err != nil {
			return BucketSnapshot{}, fmt.Errorf("message metadata: %w", err)
		}

		name, ok := objectNameFromSubject(subject, metaPrefix)
		if !ok {
			continue
		}

		var objInfo jetstream.ObjectInfo
		if err := json.Unmarshal(msg.Data(), &objInfo); err != nil {
			continue
		}
		objInfo.Name = name
		objInfo.Bucket = bucket

		next := objectState{
			seq:  meta.Sequence.Stream,
			info: objInfo,
		}
		if prev, exists := states[subject]; !exists || next.seq >= prev.seq {
			states[subject] = next
		}
		metaCount++
		if report != nil && (scanned == totalMessages || scanned%250 == 0 || time.Since(lastReport) >= 2*time.Second) {
			reportScan(scanned, len(states))
			lastReport = time.Now()
		}
	}

	reportScan(scanned, len(states))

	snap := BucketSnapshot{MessageCount: metaCount}
	for _, state := range states {
		name := state.info.Name
		snap.Listed = append(snap.Listed, name)
		if objectMetaMigratable(&state.info) {
			info := state.info
			snap.Migratable = append(snap.Migratable, &info)
		} else {
			snap.Omitted = append(snap.Omitted, name)
		}
	}

	sort.Strings(snap.Listed)
	sort.Strings(snap.Omitted)
	sort.Slice(snap.Migratable, func(i, j int) bool {
		return snap.Migratable[i].Name < snap.Migratable[j].Name
	})

	return snap, nil
}

func objectNameFromSubject(subject, prefix string) (string, bool) {
	if !strings.HasPrefix(subject, prefix) {
		return "", false
	}
	encoded := strings.TrimPrefix(subject, prefix)
	if encoded == "" {
		return "", false
	}
	nameBytes, err := base64.URLEncoding.DecodeString(encoded)
	if err != nil {
		return "", false
	}
	name := string(nameBytes)
	return name, name != ""
}

func objectMetaMigratable(info *jetstream.ObjectInfo) bool {
	if info.Deleted {
		return false
	}
	if IsLink(info) {
		return true
	}
	return info.NUID != ""
}
