package progress

import (
	"encoding/json"
	"os"
	"time"
)

func emitJSON(eventType string, fields map[string]any) {
	fields["type"] = eventType
	fields["time"] = time.Now().UTC().Format(time.RFC3339Nano)
	data, err := json.Marshal(fields)
	if err != nil {
		return
	}
	os.Stdout.Write(data)
	os.Stdout.Write([]byte("\n"))
}
