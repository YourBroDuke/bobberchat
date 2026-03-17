package adapter

import (
	"context"

	"github.com/bobberchat/bobberchat/backend/internal/protocol"
)

// Adapter is the shared contract for all protocol adapters (MCP, A2A, gRPC).
type Adapter interface {
	Name() string
	Protocol() string
	Ingest(ctx context.Context, raw []byte, meta TransportMeta) (*protocol.Envelope, error)
	Emit(ctx context.Context, env *protocol.Envelope) ([]byte, error)
	Validate(raw []byte) error
}

type TransportMeta struct {
	ConnectionID string
	SourceAddr   string
	AgentID      string
	Headers      map[string]string
}

const (
	MetaKeyAdapter        = "adapter"
	MetaKeyAdapterName    = "adapter_name"
	MetaKeyAdapterVersion = "adapter_version"
	MetaKeyDirection      = "direction"
	MetaKeySourceID       = "source_id"
	MetaKeySourceProtocol = "source_protocol"

	DirectionInbound  = "inbound"
	DirectionOutbound = "outbound"
)

func SetAdapterMetadata(env *protocol.Envelope, adapterName, version, direction, sourceID, sourceProtocol string) {
	if env.Metadata == nil {
		env.Metadata = make(map[string]any)
	}
	env.Metadata[MetaKeyAdapter] = map[string]any{
		MetaKeyAdapterName:    adapterName,
		MetaKeyAdapterVersion: version,
		MetaKeyDirection:      direction,
		MetaKeySourceID:       sourceID,
		MetaKeySourceProtocol: sourceProtocol,
	}
}
