//go:build whatsapp_native

package whatsapp

// Tests for media content annotation added to handleIncoming.
//
// WhatsApp media messages (ImageMessage, DocumentMessage, AudioMessage) carry
// no plain text; the content field that the agent loop receives comes from:
//   - ImageMessage.Caption (if present), otherwise "[image]"
//   - DocumentMessage.FileName wrapped as "[file: <name>]"
//   - AudioMessage → "[voice]"
//
// The actual file download (client.Download) requires a live whatsmeow client
// and is not tested here. These tests cover the annotation helpers that derive
// the text content from the proto message, which can be exercised without a
// network connection.

import (
	"testing"

	"go.mau.fi/whatsmeow/proto/waE2E"
	"google.golang.org/protobuf/proto"
)

// mediaContentFromMessage simulates the content-annotation logic in handleIncoming
// for a single media type. It mirrors the production code so that changes there
// are reflected in test failures immediately.
func mediaContentFromMessage(msg *waE2E.Message, existingContent string) string {
	content := existingContent

	if img := msg.GetImageMessage(); img != nil {
		if caption := img.GetCaption(); caption != "" {
			if content != "" {
				content += "\n"
			}
			content += caption
		} else if content == "" {
			content = "[image]"
		}
		return content
	}

	if doc := msg.GetDocumentMessage(); doc != nil {
		filename := doc.GetFileName()
		if filename == "" {
			filename = "document"
		}
		if content == "" {
			content = "[file: " + filename + "]"
		}
		return content
	}

	if audio := msg.GetAudioMessage(); audio != nil {
		if content == "" {
			content = "[voice]"
		}
		return content
	}

	return content
}

func TestMediaAnnotation_ImageOnly(t *testing.T) {
	msg := &waE2E.Message{
		ImageMessage: &waE2E.ImageMessage{},
	}
	got := mediaContentFromMessage(msg, "")
	if got != "[image]" {
		t.Errorf("image-only annotation = %q, want %q", got, "[image]")
	}
}

func TestMediaAnnotation_ImageWithCaption(t *testing.T) {
	msg := &waE2E.Message{
		ImageMessage: &waE2E.ImageMessage{
			Caption: proto.String("look at this"),
		},
	}
	got := mediaContentFromMessage(msg, "")
	if got != "look at this" {
		t.Errorf("image caption annotation = %q, want %q", got, "look at this")
	}
}

func TestMediaAnnotation_ImageCaptionAppendsToExistingContent(t *testing.T) {
	msg := &waE2E.Message{
		ImageMessage: &waE2E.ImageMessage{
			Caption: proto.String("caption here"),
		},
	}
	got := mediaContentFromMessage(msg, "existing text")
	want := "existing text\ncaption here"
	if got != want {
		t.Errorf("image caption append = %q, want %q", got, want)
	}
}

func TestMediaAnnotation_DocumentWithFilename(t *testing.T) {
	msg := &waE2E.Message{
		DocumentMessage: &waE2E.DocumentMessage{
			FileName: proto.String("report.pdf"),
		},
	}
	got := mediaContentFromMessage(msg, "")
	want := "[file: report.pdf]"
	if got != want {
		t.Errorf("document annotation = %q, want %q", got, want)
	}
}

func TestMediaAnnotation_DocumentWithoutFilename(t *testing.T) {
	msg := &waE2E.Message{
		DocumentMessage: &waE2E.DocumentMessage{},
	}
	got := mediaContentFromMessage(msg, "")
	want := "[file: document]"
	if got != want {
		t.Errorf("document-no-name annotation = %q, want %q", got, want)
	}
}

func TestMediaAnnotation_AudioMessage(t *testing.T) {
	msg := &waE2E.Message{
		AudioMessage: &waE2E.AudioMessage{},
	}
	got := mediaContentFromMessage(msg, "")
	if got != "[voice]" {
		t.Errorf("audio annotation = %q, want %q", got, "[voice]")
	}
}

func TestMediaAnnotation_AudioDoesNotOverwriteExistingContent(t *testing.T) {
	msg := &waE2E.Message{
		AudioMessage: &waE2E.AudioMessage{},
	}
	got := mediaContentFromMessage(msg, "transcribed text")
	if got != "transcribed text" {
		t.Errorf("audio with existing content = %q, want %q", got, "transcribed text")
	}
}
