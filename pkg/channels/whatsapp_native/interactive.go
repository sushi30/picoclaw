//go:build whatsapp_native

// PicoClaw - Ultra-lightweight personal AI agent
// License: MIT
//
// Copyright (c) 2026 PicoClaw contributors

package whatsapp

import (
	"context"
	"encoding/json"
	"fmt"

	"go.mau.fi/whatsmeow/proto/waE2E"
	"google.golang.org/protobuf/proto"

	"github.com/sipeed/picoclaw/pkg/bus"
	"github.com/sipeed/picoclaw/pkg/channels"
)

// Metadata keys written by the agent (outbound) and read by Send().
// These are internal to the whatsapp_native package.
const (
	metaKeyInteractiveType = "wa_interactive_type" // "buttons" | "list"
	metaKeyButtonsJSON     = "wa_buttons_json"     // JSON-encoded []channels.InteractiveButton
	metaKeyListJSON        = "wa_list_json"        // JSON-encoded []channels.InteractiveSection
	metaKeyListButtonLabel = "wa_list_button"      // label for the list-open button
)

// buildInteractiveMessage inspects msg.Metadata for interactive hints and
// returns the appropriate waE2E.Message, or (nil, nil) if none are present
// (triggering the plain-text fallback in Send).
func buildInteractiveMessage(msg bus.OutboundMessage) (*waE2E.Message, error) {
	switch msg.Metadata[metaKeyInteractiveType] {
	case "buttons":
		var btns []channels.InteractiveButton
		if err := json.Unmarshal([]byte(msg.Metadata[metaKeyButtonsJSON]), &btns); err != nil {
			return nil, fmt.Errorf("parse buttons json: %w", err)
		}
		return buildButtonsMessage(msg.Content, btns), nil
	case "list":
		var sections []channels.InteractiveSection
		if err := json.Unmarshal([]byte(msg.Metadata[metaKeyListJSON]), &sections); err != nil {
			return nil, fmt.Errorf("parse list json: %w", err)
		}
		label := msg.Metadata[metaKeyListButtonLabel]
		if label == "" {
			label = "Choose"
		}
		return buildListMessage(msg.Content, label, sections), nil
	default:
		return nil, nil
	}
}

func buildButtonsMessage(body string, buttons []channels.InteractiveButton) *waE2E.Message {
	waButtons := make([]*waE2E.Button, len(buttons))
	for i, b := range buttons {
		waButtons[i] = &waE2E.Button{
			ButtonId: proto.String(b.ID),
			ButtonText: &waE2E.Button_ButtonText{
				DisplayText: proto.String(b.Title),
			},
			Type: waE2E.Button_RESPONSE.Enum(),
		}
	}
	return &waE2E.Message{
		ButtonsMessage: &waE2E.ButtonsMessage{
			ContentText: proto.String(body),
			Buttons:     waButtons,
			HeaderType:  waE2E.ButtonsMessage_EMPTY.Enum(),
		},
	}
}

func buildListMessage(body, buttonLabel string, sections []channels.InteractiveSection) *waE2E.Message {
	waSections := make([]*waE2E.ListMessage_Section, len(sections))
	for i, s := range sections {
		rows := make([]*waE2E.ListMessage_Row, len(s.Rows))
		for j, r := range s.Rows {
			rows[j] = &waE2E.ListMessage_Row{
				RowId:       proto.String(r.ID),
				Title:       proto.String(r.Title),
				Description: proto.String(r.Description),
			}
		}
		waSections[i] = &waE2E.ListMessage_Section{
			Title: proto.String(s.Title),
			Rows:  rows,
		}
	}
	return &waE2E.Message{
		ListMessage: &waE2E.ListMessage{
			Description: proto.String(body),
			ButtonText:  proto.String(buttonLabel),
			ListType:    waE2E.ListMessage_SINGLE_SELECT.Enum(),
			Sections:    waSections,
		},
	}
}

// SendButtons implements channels.InteractiveCapable.
// It sends a message with up to 3 quick-reply buttons directly via the
// WhatsApp client (not through the bus/manager). WhatsApp enforces a hard
// limit of 3 buttons per message; callers are responsible for truncating.
func (c *WhatsAppNativeChannel) SendButtons(
	ctx context.Context,
	chatID, body string,
	buttons []channels.InteractiveButton,
) error {
	c.mu.Lock()
	client := c.client
	c.mu.Unlock()

	if client == nil || !client.IsConnected() {
		return fmt.Errorf("whatsapp not connected")
	}

	to, err := parseJID(chatID)
	if err != nil {
		return fmt.Errorf("invalid chat id %q: %w", chatID, err)
	}

	waMsg := buildButtonsMessage(body, buttons)
	if _, err = client.SendMessage(ctx, to, waMsg); err != nil {
		return fmt.Errorf("send buttons: %w", err)
	}
	return nil
}

// SendList implements channels.InteractiveCapable.
// It sends a list-picker message with one or more sections of selectable rows.
func (c *WhatsAppNativeChannel) SendList(
	ctx context.Context,
	chatID, body, buttonLabel string,
	sections []channels.InteractiveSection,
) error {
	c.mu.Lock()
	client := c.client
	c.mu.Unlock()

	if client == nil || !client.IsConnected() {
		return fmt.Errorf("whatsapp not connected")
	}

	to, err := parseJID(chatID)
	if err != nil {
		return fmt.Errorf("invalid chat id %q: %w", chatID, err)
	}

	waMsg := buildListMessage(body, buttonLabel, sections)
	if _, err = client.SendMessage(ctx, to, waMsg); err != nil {
		return fmt.Errorf("send list: %w", err)
	}
	return nil
}
