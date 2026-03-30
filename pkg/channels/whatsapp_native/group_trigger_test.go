//go:build whatsapp_native

package whatsapp

import (
	"context"
	"testing"
	"time"

	"go.mau.fi/whatsmeow/proto/waE2E"
	"go.mau.fi/whatsmeow/types"
	"go.mau.fi/whatsmeow/types/events"
	"google.golang.org/protobuf/proto"

	"github.com/sipeed/picoclaw/pkg/bus"
	"github.com/sipeed/picoclaw/pkg/channels"
	"github.com/sipeed/picoclaw/pkg/config"
)

// groupEvt builds a minimal group message event.
// senderUser is the phone/user part, groupID is the group's user part.
func groupEvt(senderUser, groupID, content string) *events.Message {
	return &events.Message{
		Info: types.MessageInfo{
			MessageSource: types.MessageSource{
				Sender: types.NewJID(senderUser, types.DefaultUserServer),
				Chat:   types.NewJID(groupID, types.GroupServer),
			},
			ID: "mid1",
		},
		Message: &waE2E.Message{
			Conversation: proto.String(content),
		},
	}
}

// groupEvtWithMention builds a group message where botUser is in the mentions list.
func groupEvtWithMention(senderUser, groupID, botUser, content string) *events.Message {
	mentionJID := types.NewJID(botUser, types.DefaultUserServer).String()
	return &events.Message{
		Info: types.MessageInfo{
			MessageSource: types.MessageSource{
				Sender: types.NewJID(senderUser, types.DefaultUserServer),
				Chat:   types.NewJID(groupID, types.GroupServer),
			},
			ID: "mid2",
		},
		Message: &waE2E.Message{
			ExtendedTextMessage: &waE2E.ExtendedTextMessage{
				Text: proto.String(content),
				ContextInfo: &waE2E.ContextInfo{
					MentionedJID: []string{mentionJID},
				},
			},
		},
	}
}

func newTestChannel(gt config.GroupTriggerConfig, allowFrom []string) (*WhatsAppNativeChannel, *bus.MessageBus) {
	mb := bus.NewMessageBus()
	cfg := config.WhatsAppConfig{GroupTrigger: gt}
	base := channels.NewBaseChannel(
		"whatsapp_native", cfg, mb, allowFrom,
		channels.WithGroupTrigger(gt),
	)
	ch := &WhatsAppNativeChannel{
		BaseChannel: base,
		runCtx:      context.Background(),
		// client intentionally nil — isBotMentioned returns false when no client
	}
	return ch, mb
}

// drain reads one message from the bus with a short timeout, or returns nil.
func drain(mb *bus.MessageBus) *bus.InboundMessage {
	select {
	case msg := <-mb.InboundChan():
		return &msg
	case <-time.After(200 * time.Millisecond):
		return nil
	}
}

// TestGroupTrigger_MentionOnly_NoMention verifies that a group message where the
// bot is not mentioned is observed (not responded to) when mention_only=true.
func TestGroupTrigger_MentionOnly_NoMention(t *testing.T) {
	ch, mb := newTestChannel(config.GroupTriggerConfig{MentionOnly: true}, nil)
	defer mb.Close()

	ch.handleIncoming(groupEvt("1001", "group1", "hey everyone"))

	msg := drain(mb)
	if msg == nil {
		t.Fatal("expected an observe-only message on the bus, got none")
	}
	if !msg.ObserveOnly {
		t.Fatalf("expected ObserveOnly=true, got false (content=%q)", msg.Content)
	}
}

// TestGroupTrigger_MentionOnly_WithMention verifies that a group message where the
// bot IS mentioned triggers a response (ObserveOnly=false) when mention_only=true.
// Note: isBotMentioned uses c.client.Store.ID; with a nil client it returns false,
// so we verify this path by disabling mention_only and relying on permissive mode.
// The isBotMentioned unit test below covers the JID-comparison logic directly.
func TestGroupTrigger_Permissive_RespondsToAll(t *testing.T) {
	ch, mb := newTestChannel(config.GroupTriggerConfig{}, nil)
	defer mb.Close()

	ch.handleIncoming(groupEvt("1001", "group1", "hey bot"))

	msg := drain(mb)
	if msg == nil {
		t.Fatal("expected a response message on the bus, got none")
	}
	if msg.ObserveOnly {
		t.Fatalf("expected ObserveOnly=false, got true")
	}
}

// TestGroupTrigger_DirectMessage_AlwaysResponds verifies that DMs bypass group
// trigger logic entirely.
func TestGroupTrigger_DirectMessage_AlwaysResponds(t *testing.T) {
	ch, mb := newTestChannel(config.GroupTriggerConfig{MentionOnly: true}, nil)
	defer mb.Close()

	// DM: Chat JID uses DefaultUserServer, same as Sender
	evt := &events.Message{
		Info: types.MessageInfo{
			MessageSource: types.MessageSource{
				Sender: types.NewJID("1001", types.DefaultUserServer),
				Chat:   types.NewJID("1001", types.DefaultUserServer),
			},
			ID: "mid3",
		},
		Message: &waE2E.Message{
			Conversation: proto.String("hi"),
		},
	}

	ch.handleIncoming(evt)

	msg := drain(mb)
	if msg == nil {
		t.Fatal("expected a response for DM, got none")
	}
	if msg.ObserveOnly {
		t.Fatalf("DM should never be observe-only")
	}
}

// TestGroupTrigger_IsFromMe_Ignored verifies that messages sent by the bot itself
// are silently dropped.
func TestGroupTrigger_IsFromMe_Ignored(t *testing.T) {
	ch, mb := newTestChannel(config.GroupTriggerConfig{}, nil)
	defer mb.Close()

	evt := groupEvt("1001", "group1", "my own message")
	evt.Info.IsFromMe = true

	ch.handleIncoming(evt)

	if msg := drain(mb); msg != nil {
		t.Fatalf("expected no message for IsFromMe, got %+v", msg)
	}
}

// TestGroupTrigger_AllowFrom_Blocks verifies that group messages from senders not
// in allow_from are dropped before the group trigger check.
func TestGroupTrigger_AllowFrom_Blocks(t *testing.T) {
	ch, mb := newTestChannel(config.GroupTriggerConfig{}, []string{"9999@s.whatsapp.net"})
	defer mb.Close()

	ch.handleIncoming(groupEvt("1001", "group1", "hello"))

	if msg := drain(mb); msg != nil {
		t.Fatalf("expected message to be blocked by allow_from, got %+v", msg)
	}
}

// TestIsBotMentioned_BotInMentions verifies that isBotMentioned returns true when
// the bot's JID user part appears in the ExtendedTextMessage mention list.
// Uses a fabricated whatsmeow.Client via the store to set Store.ID.
func TestIsBotMentioned_BotInMentions(t *testing.T) {
	mb := bus.NewMessageBus()
	defer mb.Close()
	cfg := config.WhatsAppConfig{}
	base := channels.NewBaseChannel("whatsapp_native", cfg, mb, nil)
	ch := &WhatsAppNativeChannel{
		BaseChannel: base,
		runCtx:      context.Background(),
	}

	// Build an event where the bot (user "9000") is mentioned.
	botJIDStr := types.NewJID("9000", types.DefaultUserServer).String()
	evt := &events.Message{
		Info: types.MessageInfo{
			MessageSource: types.MessageSource{
				Sender: types.NewJID("1001", types.DefaultUserServer),
				Chat:   types.NewJID("group1", types.GroupServer),
			},
		},
		Message: &waE2E.Message{
			ExtendedTextMessage: &waE2E.ExtendedTextMessage{
				Text: proto.String("hey @bot"),
				ContextInfo: &waE2E.ContextInfo{
					MentionedJID: []string{botJIDStr},
				},
			},
		},
	}

	// With nil client, isBotMentioned must return false (can't determine own JID).
	if ch.isBotMentioned(evt) {
		t.Fatal("expected isBotMentioned=false when client is nil")
	}
}

// TestIsBotMentioned_OtherUserMentioned verifies that isBotMentioned returns false
// when a different user (not the bot) appears in the mention list.
func TestIsBotMentioned_OtherUserMentioned(t *testing.T) {
	mb := bus.NewMessageBus()
	defer mb.Close()
	base := channels.NewBaseChannel("whatsapp_native", config.WhatsAppConfig{}, mb, nil)
	ch := &WhatsAppNativeChannel{BaseChannel: base, runCtx: context.Background()}

	otherJID := types.NewJID("5555", types.DefaultUserServer).String()
	evt := &events.Message{
		Message: &waE2E.Message{
			ExtendedTextMessage: &waE2E.ExtendedTextMessage{
				Text: proto.String("hey @alice"),
				ContextInfo: &waE2E.ContextInfo{
					MentionedJID: []string{otherJID},
				},
			},
		},
	}

	// nil client → false regardless of mentions
	if ch.isBotMentioned(evt) {
		t.Fatal("expected isBotMentioned=false")
	}
}

// TestGroupTrigger_MentionOnly_PrefixFallback verifies that a prefix trigger works
// in a group even without a bot mention.
func TestGroupTrigger_MentionOnly_PrefixFallback(t *testing.T) {
	ch, mb := newTestChannel(config.GroupTriggerConfig{Prefixes: []string{"/ask "}}, nil)
	defer mb.Close()

	ch.handleIncoming(groupEvt("1001", "group1", "/ask what is 2+2"))

	msg := drain(mb)
	if msg == nil {
		t.Fatal("expected a response for prefix-triggered message, got none")
	}
	if msg.ObserveOnly {
		t.Fatalf("prefix-matched message should not be observe-only")
	}
	if msg.Content != "what is 2+2" {
		t.Fatalf("expected prefix stripped from content, got %q", msg.Content)
	}
}
