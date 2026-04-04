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

// newVoiceTestChannel builds a minimal WhatsAppNativeChannel for unit tests.
// The whatsmeow client is intentionally left nil; downloadVoice gracefully returns ""
// when the client is not ready.
func newVoiceTestChannel(cfg config.WhatsAppConfig) (*WhatsAppNativeChannel, *bus.MessageBus) {
	mb := bus.NewMessageBus()
	ch := &WhatsAppNativeChannel{
		BaseChannel: channels.NewBaseChannel("whatsapp_native", cfg, mb, nil),
		config:      cfg,
		runCtx:      context.Background(),
	}
	return ch, mb
}

func TestVoiceCapabilities(t *testing.T) {
	ch, _ := newVoiceTestChannel(config.WhatsAppConfig{})
	caps := ch.VoiceCapabilities()
	if !caps.ASR {
		t.Error("expected ASR=true")
	}
	if caps.TTS {
		t.Error("expected TTS=false")
	}
}

// TestHandleIncoming_VoiceOnlyDroppedWhenClientNil verifies that a voice-only message
// (no text content) is silently dropped when the whatsmeow client is not initialised.
// This guards against panics and ensures the early-return path is correct.
func TestHandleIncoming_VoiceOnlyDroppedWhenClientNil(t *testing.T) {
	ch, mb := newVoiceTestChannel(config.WhatsAppConfig{})

	evt := &events.Message{
		Info: types.MessageInfo{
			MessageSource: types.MessageSource{
				Sender: types.NewJID("1001", types.DefaultUserServer),
				Chat:   types.NewJID("1001", types.DefaultUserServer),
			},
			ID: "voice-msg-1",
		},
		Message: &waE2E.Message{
			AudioMessage: &waE2E.AudioMessage{
				PTT:      proto.Bool(true),
				Mimetype: proto.String("audio/ogg; codecs=opus"),
			},
		},
	}

	ch.handleIncoming(evt)

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	select {
	case msg := <-mb.InboundChan():
		t.Errorf("expected no message published for voice-only with nil client, got: %v", msg)
	case <-ctx.Done():
		// correct: dropped
	}
}

// TestHandleIncoming_TextWithVoiceClientNil verifies that when a message has both text
// and an AudioMessage, and the client is nil, the text is still published but no media
// ref is attached (the download was skipped).
func TestHandleIncoming_TextWithVoiceClientNil(t *testing.T) {
	ch, mb := newVoiceTestChannel(config.WhatsAppConfig{})

	evt := &events.Message{
		Info: types.MessageInfo{
			MessageSource: types.MessageSource{
				Sender: types.NewJID("1001", types.DefaultUserServer),
				Chat:   types.NewJID("1001", types.DefaultUserServer),
			},
			ID: "text-voice-msg-1",
		},
		Message: &waE2E.Message{
			Conversation: proto.String("hello"),
			AudioMessage: &waE2E.AudioMessage{
				PTT:      proto.Bool(true),
				Mimetype: proto.String("audio/ogg; codecs=opus"),
			},
		},
	}

	ch.handleIncoming(evt)

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	select {
	case msg := <-mb.InboundChan():
		if msg.Content != "hello" {
			t.Errorf("expected content %q, got %q", "hello", msg.Content)
		}
		if len(msg.Media) != 0 {
			t.Errorf("expected no media refs (download skipped), got %d", len(msg.Media))
		}
	case <-ctx.Done():
		t.Error("timeout: expected an inbound message to be published")
	}
}

// TestHandleIncoming_EchoTranscriptionNotSetForTextOnly verifies that the
// echo_transcription metadata key is NOT set when no voice media was downloaded.
func TestHandleIncoming_EchoTranscriptionNotSetForTextOnly(t *testing.T) {
	cfg := config.WhatsAppConfig{EchoTranscription: true}
	ch, mb := newVoiceTestChannel(cfg)

	evt := &events.Message{
		Info: types.MessageInfo{
			MessageSource: types.MessageSource{
				Sender: types.NewJID("1001", types.DefaultUserServer),
				Chat:   types.NewJID("1001", types.DefaultUserServer),
			},
			ID: "echo-test-1",
		},
		Message: &waE2E.Message{
			Conversation: proto.String("plain text"),
		},
	}

	ch.handleIncoming(evt)

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	select {
	case msg := <-mb.InboundChan():
		if msg.Metadata["echo_transcription"] == "true" {
			t.Error("echo_transcription should not be set for text-only messages")
		}
	case <-ctx.Done():
		t.Error("timeout: expected an inbound message to be published")
	}
}
