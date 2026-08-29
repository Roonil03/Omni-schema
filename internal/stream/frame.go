package stream

import "omni-schema/internal/network"

// DeliveryMode is at-most-once / best-effort. Bounded queues use DropOldest.
// Replay/resume is not implemented: cursors are informational only.
const DeliveryMode = "at-most-once/best-effort"
const ReplayMode = "none"

type Frame struct {
	Opcode  network.Opcode
	Payload []byte
}

func isTextFormat(format string) bool {
	switch format {
	case "graphql", "json", "odata", "graphql_sdl":
		return true
	default:
		return false
	}
}
