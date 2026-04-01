package whatsapp

import (
	"context"
	"testing"

	"github.com/sipeed/picoclaw/pkg/bus"
	"github.com/sipeed/picoclaw/pkg/channels"
	"github.com/sipeed/picoclaw/pkg/config"
)

func newTestChannel(gt config.GroupTriggerConfig) (*WhatsAppChannel, *bus.MessageBus) {
	messageBus := bus.NewMessageBus()
	cfg := config.WhatsAppConfig{GroupTrigger: gt}
	ch := &WhatsAppChannel{
		BaseChannel: channels.NewBaseChannel("whatsapp", cfg, messageBus, nil,
			channels.WithGroupTrigger(cfg.GroupTrigger),
		),
		ctx: context.Background(),
	}
	return ch, messageBus
}

// drainInbound returns the next non-observe-only message from the bus, if any.
// ObserveOnly messages (recorded by ObserveGroupMessage) are not counted as "forwarded".
func drainInbound(mb *bus.MessageBus) (bus.InboundMessage, bool) {
	select {
	case msg := <-mb.InboundChan():
		if msg.ObserveOnly {
			return bus.InboundMessage{}, false
		}
		return msg, true
	default:
		return bus.InboundMessage{}, false
	}
}

func TestHandleIncomingMessage_DoesNotConsumeGenericCommandsLocally(t *testing.T) {
	messageBus := bus.NewMessageBus()
	ch := &WhatsAppChannel{
		BaseChannel: channels.NewBaseChannel("whatsapp", config.WhatsAppConfig{}, messageBus, nil),
		ctx:         context.Background(),
	}

	ch.handleIncomingMessage(map[string]any{
		"type":    "message",
		"id":      "mid1",
		"from":    "user1",
		"chat":    "chat1",
		"content": "/help",
	})

	inbound, ok := <-messageBus.InboundChan()
	if !ok {
		t.Fatal("expected inbound message to be forwarded")
	}
	if inbound.Channel != "whatsapp" {
		t.Fatalf("channel=%q", inbound.Channel)
	}
	if inbound.Content != "/help" {
		t.Fatalf("content=%q", inbound.Content)
	}
}

// TestGroupTrigger_MentionOnly verifies that with mention_only:true the bot
// ignores group messages unless the bridge signals a mention.
func TestGroupTrigger_MentionOnly(t *testing.T) {
	gt := config.GroupTriggerConfig{MentionOnly: true}

	t.Run("group message without mention is dropped", func(t *testing.T) {
		ch, mb := newTestChannel(gt)
		ch.handleIncomingMessage(map[string]any{
			"from":    "user1",
			"chat":    "group1",
			"content": "hello everyone",
		})
		if _, ok := drainInbound(mb); ok {
			t.Fatal("expected message to be dropped, but it was forwarded")
		}
	})

	t.Run("group message with mentioned:true is forwarded", func(t *testing.T) {
		ch, mb := newTestChannel(gt)
		ch.handleIncomingMessage(map[string]any{
			"from":      "user1",
			"chat":      "group1",
			"content":   "hey bot",
			"mentioned": true,
		})
		msg, ok := drainInbound(mb)
		if !ok {
			t.Fatal("expected message to be forwarded, but it was dropped")
		}
		if msg.Content != "hey bot" {
			t.Fatalf("content=%q", msg.Content)
		}
	})

	t.Run("group message with non-empty mentions array is forwarded", func(t *testing.T) {
		ch, mb := newTestChannel(gt)
		ch.handleIncomingMessage(map[string]any{
			"from":     "user1",
			"chat":     "group1",
			"content":  "hey bot",
			"mentions": []any{"botjid@s.whatsapp.net"},
		})
		if _, ok := drainInbound(mb); !ok {
			t.Fatal("expected message to be forwarded, but it was dropped")
		}
	})

	t.Run("DM is always forwarded regardless of mention_only", func(t *testing.T) {
		ch, mb := newTestChannel(gt)
		ch.handleIncomingMessage(map[string]any{
			"from":    "user1",
			"chat":    "user1", // chat == sender → DM
			"content": "private message",
		})
		if _, ok := drainInbound(mb); !ok {
			t.Fatal("expected DM to be forwarded, but it was dropped")
		}
	})
}

// TestGroupTrigger_Prefix verifies prefix-based group trigger filtering.
func TestGroupTrigger_Prefix(t *testing.T) {
	gt := config.GroupTriggerConfig{Prefixes: []string{"/ask"}}

	t.Run("group message with matching prefix is forwarded and prefix stripped", func(t *testing.T) {
		ch, mb := newTestChannel(gt)
		ch.handleIncomingMessage(map[string]any{
			"from":    "user1",
			"chat":    "group1",
			"content": "/ask what time is it",
		})
		msg, ok := drainInbound(mb)
		if !ok {
			t.Fatal("expected message to be forwarded")
		}
		if msg.Content != "what time is it" {
			t.Fatalf("content=%q, want %q", msg.Content, "what time is it")
		}
	})

	t.Run("group message without prefix is dropped", func(t *testing.T) {
		ch, mb := newTestChannel(gt)
		ch.handleIncomingMessage(map[string]any{
			"from":    "user1",
			"chat":    "group1",
			"content": "hello",
		})
		if _, ok := drainInbound(mb); ok {
			t.Fatal("expected message to be dropped")
		}
	})
}

// TestGroupTrigger_NoConfig verifies that without any group_trigger config the
// bot responds to all group messages (permissive default).
func TestGroupTrigger_NoConfig(t *testing.T) {
	ch, mb := newTestChannel(config.GroupTriggerConfig{})
	ch.handleIncomingMessage(map[string]any{
		"from":    "user1",
		"chat":    "group1",
		"content": "any message",
	})
	if _, ok := drainInbound(mb); !ok {
		t.Fatal("expected message to be forwarded with no group_trigger config")
	}
}
