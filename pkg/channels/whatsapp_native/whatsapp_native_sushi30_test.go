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

// TestWhatsAppNativeChannel_VoiceCapabilities verifies ASR is declared, TTS is not.
func TestWhatsAppNativeChannel_VoiceCapabilities(t *testing.T) {
	ch := &WhatsAppNativeChannel{
		BaseChannel: channels.NewBaseChannel("whatsapp_native", config.WhatsAppConfig{}, bus.NewMessageBus(), nil),
	}
	caps := ch.VoiceCapabilities()
	if !caps.ASR {
		t.Error("expected ASR = true")
	}
	if caps.TTS {
		t.Error("expected TTS = false")
	}
}

// TestHandleIncoming_AudioOnly_NilClient verifies that receiving an audio-only
// message with no connected client does not panic and does not forward a message.
func TestHandleIncoming_AudioOnly_NilClient(t *testing.T) {
	msgBus := bus.NewMessageBus()
	ch := &WhatsAppNativeChannel{
		BaseChannel: channels.NewBaseChannel("whatsapp_native", config.WhatsAppConfig{}, msgBus, nil),
		runCtx:      context.Background(),
		// client is intentionally nil — simulates pre-connect state
	}

	ptt := true
	evt := &events.Message{
		Info: types.MessageInfo{
			MessageSource: types.MessageSource{
				Sender: types.JID{User: "15550001111", Server: types.DefaultUserServer},
				Chat:   types.JID{User: "15550001111", Server: types.DefaultUserServer},
			},
			ID:        "test-msg-id",
			Timestamp: time.Now(),
		},
		Message: &waE2E.Message{
			AudioMessage: &waE2E.AudioMessage{
				PTT: proto.Bool(ptt),
			},
		},
	}

	ch.handleIncoming(evt)

	// No message should be forwarded because the client is nil (download fails).
	select {
	case <-msgBus.InboundChan():
		t.Error("expected no message forwarded for audio-only event with nil client")
	default:
		// Correct: nothing forwarded.
	}
}

// TestHandleIncoming_TextMessage_StillWorks verifies that plain text messages
// are unaffected by the voice note changes.
func TestHandleIncoming_TextMessage_StillWorks(t *testing.T) {
	msgBus := bus.NewMessageBus()
	ch := &WhatsAppNativeChannel{
		BaseChannel: channels.NewBaseChannel(
			"whatsapp_native",
			config.WhatsAppConfig{},
			msgBus,
			nil, // allow all senders
		),
		runCtx: context.Background(),
	}

	evt := &events.Message{
		Info: types.MessageInfo{
			MessageSource: types.MessageSource{
				Sender: types.JID{User: "15550002222", Server: types.DefaultUserServer},
				Chat:   types.JID{User: "15550002222", Server: types.DefaultUserServer},
			},
			ID:        "text-msg-id",
			Timestamp: time.Now(),
		},
		Message: &waE2E.Message{
			Conversation: proto.String("hello"),
		},
	}

	ch.handleIncoming(evt)

	select {
	case msg := <-msgBus.InboundChan():
		if msg.Content != "hello" {
			t.Errorf("expected content %q, got %q", "hello", msg.Content)
		}
	case <-time.After(100 * time.Millisecond):
		t.Error("expected a message to be forwarded but got none")
	}
}
