package broker

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/bobberchat/bobberchat/internal/observability"
	"github.com/bobberchat/bobberchat/internal/persistence"
	"github.com/bobberchat/bobberchat/internal/protocol"
	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
)

type Broker struct {
	nc      *nats.Conn
	js      jetstream.JetStream
	db      *persistence.DB
	metrics *observability.Metrics
}

func NewBroker(natsURL string, db *persistence.DB, metrics *observability.Metrics) (*Broker, error) {
	if strings.TrimSpace(natsURL) == "" {
		return nil, errors.New("nats url required")
	}

	nc, err := nats.Connect(natsURL)
	if err != nil {
		return nil, fmt.Errorf("connect nats: %w", err)
	}
	js, err := jetstream.New(nc)
	if err != nil {
		nc.Close()
		return nil, fmt.Errorf("init jetstream: %w", err)
	}

	return &Broker{nc: nc, js: js, db: db, metrics: metrics}, nil
}

func (b *Broker) Setup(ctx context.Context) error {
	if b == nil || b.js == nil {
		return errors.New("broker not initialized")
	}

	streams := []jetstream.StreamConfig{
		{
			Name:      "BOBBER_MSG",
			Subjects:  []string{"bobberchat.*.msg.>", "bobberchat.*.group.>"},
			Retention: jetstream.InterestPolicy,
			MaxAge:    30 * 24 * time.Hour,
		},
		{
			Name:      "BOBBER_SYSTEM",
			Subjects:  []string{"bobberchat.*.system.>"},
			Retention: jetstream.LimitsPolicy,
			MaxAge:    24 * time.Hour,
		},
		{
			Name:      "BOBBER_APPROVAL",
			Subjects:  []string{"bobberchat.*.approval.>"},
			Retention: jetstream.WorkQueuePolicy,
			MaxAge:    7 * 24 * time.Hour,
		},
	}

	for _, cfg := range streams {
		if _, err := b.js.CreateOrUpdateStream(ctx, cfg); err != nil {
			return fmt.Errorf("create/update stream %s: %w", cfg.Name, err)
		}
	}

	return nil
}

func (b *Broker) PublishMessage(ctx context.Context, env *protocol.Envelope) error {
	if b == nil || b.js == nil || env == nil {
		return errors.New("invalid broker publish input")
	}
	if err := env.Validate(); err != nil {
		return err
	}

	subject, err := subjectForEnvelope(env)
	if err != nil {
		return err
	}

	data, err := json.Marshal(env)
	if err != nil {
		return fmt.Errorf("marshal envelope: %w", err)
	}

	msg := &nats.Msg{Subject: subject, Data: data, Header: nats.Header{}}
	msg.Header.Set("Nats-Msg-Id", env.ID)
	if env.TraceID != "" {
		msg.Header.Set("trace_id", env.TraceID)
	}

	start := time.Now()
	_, err = b.js.PublishMsg(ctx, msg)
	if err != nil {
		if b.metrics != nil {
			b.metrics.ErrorsCount.WithLabelValues(env.To, "publish_error").Inc()
		}
		return fmt.Errorf("publish message: %w", err)
	}

	if b.metrics != nil {
		b.metrics.MessagesSent.WithLabelValues(env.To, env.Tag).Inc()
		b.metrics.MessagesLatency.WithLabelValues(env.To).Observe(float64(time.Since(start).Milliseconds()))
	}

	return nil
}

func (b *Broker) SubscribeAgent(ctx context.Context, tenantID, agentID string, handler func(*protocol.Envelope)) error {
	if b == nil || b.js == nil || tenantID == "" || agentID == "" || handler == nil {
		return errors.New("invalid subscribe agent input")
	}

	subject := fmt.Sprintf("bobberchat.%s.msg.%s", tenantID, agentID)
	consumerName := fmt.Sprintf("agent-%s-%s", tenantID, agentID)
	stream, err := b.js.Stream(ctx, "BOBBER_MSG")
	if err != nil {
		return err
	}

	consumer, err := stream.CreateOrUpdateConsumer(ctx, jetstream.ConsumerConfig{
		Durable:       consumerName,
		AckPolicy:     jetstream.AckExplicitPolicy,
		AckWait:       30 * time.Second,
		ReplayPolicy:  jetstream.ReplayInstantPolicy,
		FilterSubject: subject,
	})
	if err != nil {
		return err
	}

	_, err = consumer.Consume(func(msg jetstream.Msg) {
		defer msg.Ack()
		env := &protocol.Envelope{}
		if err := json.Unmarshal(msg.Data(), env); err != nil {
			return
		}
		handler(env)
	})
	if err != nil {
		return err
	}

	return nil
}

func (b *Broker) SubscribeGroup(ctx context.Context, tenantID, groupID string, handler func(*protocol.Envelope)) error {
	if b == nil || b.js == nil || tenantID == "" || groupID == "" || handler == nil {
		return errors.New("invalid subscribe group input")
	}

	subject := fmt.Sprintf("bobberchat.%s.group.%s", tenantID, groupID)
	consumerName := fmt.Sprintf("group-%s-%s", tenantID, groupID)
	stream, err := b.js.Stream(ctx, "BOBBER_MSG")
	if err != nil {
		return err
	}

	consumer, err := stream.CreateOrUpdateConsumer(ctx, jetstream.ConsumerConfig{
		Durable:       consumerName,
		AckPolicy:     jetstream.AckExplicitPolicy,
		AckWait:       30 * time.Second,
		ReplayPolicy:  jetstream.ReplayInstantPolicy,
		FilterSubject: subject,
	})
	if err != nil {
		return err
	}

	_, err = consumer.Consume(func(msg jetstream.Msg) {
		defer msg.Ack()
		env := &protocol.Envelope{}
		if err := json.Unmarshal(msg.Data(), env); err != nil {
			return
		}
		handler(env)
	})
	if err != nil {
		return err
	}

	return nil
}

func (b *Broker) Close() {
	if b == nil || b.nc == nil {
		return
	}
	b.nc.Drain()
	b.nc.Close()
}

func subjectForEnvelope(env *protocol.Envelope) (string, error) {
	tenantID, ok := env.Metadata["tenant_id"].(string)
	if !ok || tenantID == "" {
		if t, ok := env.Metadata["tenant"].(string); ok {
			tenantID = t
		}
	}
	if tenantID == "" {
		return "", errors.New("tenant_id missing in metadata")
	}

	family := protocol.ParseTagFamily(env.Tag)
	switch family {
	case protocol.TagSystem:
		suffix := strings.TrimPrefix(env.Tag, protocol.TagSystem+".")
		return fmt.Sprintf("bobberchat.%s.system.%s", tenantID, suffix), nil
	case protocol.TagApproval:
		suffix := strings.TrimPrefix(env.Tag, protocol.TagApproval+".")
		return fmt.Sprintf("bobberchat.%s.approval.%s", tenantID, suffix), nil
	default:
		if strings.HasPrefix(env.To, "group:") {
			groupID := strings.TrimPrefix(env.To, "group:")
			if groupID == "" {
				return "", errors.New("group id missing")
			}
			return fmt.Sprintf("bobberchat.%s.group.%s", tenantID, groupID), nil
		}
		return fmt.Sprintf("bobberchat.%s.msg.%s", tenantID, env.To), nil
	}
}
