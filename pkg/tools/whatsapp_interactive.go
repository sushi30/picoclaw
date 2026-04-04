package tools

import (
	"context"
	"encoding/json"
	"fmt"
)

// SendInteractiveCallback is the function type injected into WhatsApp
// interactive tools so they can publish outbound messages with widget metadata.
// The metadata map must contain "wa_interactive_type" and the corresponding
// payload key ("wa_buttons_json" or "wa_list_json").
type SendInteractiveCallback func(channel, chatID, content string, metadata map[string]string) error

// SendWhatsAppButtonsTool lets the LLM send a WhatsApp message with up to 3
// quick-reply buttons. The user's tap is delivered back as a normal inbound
// message whose Content is the tapped button ID.
type SendWhatsAppButtonsTool struct {
	sendCallback SendInteractiveCallback
}

func NewSendWhatsAppButtonsTool() *SendWhatsAppButtonsTool {
	return &SendWhatsAppButtonsTool{}
}

func (t *SendWhatsAppButtonsTool) SetSendCallback(cb SendInteractiveCallback) {
	t.sendCallback = cb
}

func (t *SendWhatsAppButtonsTool) Name() string {
	return "send_whatsapp_buttons"
}

func (t *SendWhatsAppButtonsTool) Description() string {
	return "Send a WhatsApp message with up to 3 quick-reply buttons so the user can tap a choice. " +
		"The user's selection is delivered back as the next inbound message with Content equal to the tapped button id. " +
		"Use this for yes/no questions, approval requests, or any small fixed-choice preference."
}

func (t *SendWhatsAppButtonsTool) Parameters() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"body": map[string]any{
				"type":        "string",
				"description": "Message text displayed above the buttons.",
			},
			"buttons": map[string]any{
				"type":        "array",
				"description": "List of buttons to show (max 3). The id is returned when the user taps; keep it short and code-friendly (e.g. \"yes\", \"pdf\").",
				"items": map[string]any{
					"type": "object",
					"properties": map[string]any{
						"id": map[string]any{
							"type":        "string",
							"description": "Unique button identifier echoed back as inbound Content when tapped. Max ~20 characters.",
						},
						"title": map[string]any{
							"type":        "string",
							"description": "Button label shown to the user.",
						},
					},
					"required": []string{"id", "title"},
				},
				"maxItems": 3,
			},
			"channel": map[string]any{
				"type":        "string",
				"description": "Optional: target channel (defaults to current conversation channel).",
			},
			"chat_id": map[string]any{
				"type":        "string",
				"description": "Optional: target chat/user ID (defaults to current chat).",
			},
		},
		"required": []string{"body", "buttons"},
	}
}

func (t *SendWhatsAppButtonsTool) Execute(ctx context.Context, args map[string]any) *ToolResult {
	body, ok := args["body"].(string)
	if !ok || body == "" {
		return &ToolResult{ForLLM: "body is required", IsError: true}
	}

	rawButtons, ok := args["buttons"].([]any)
	if !ok || len(rawButtons) == 0 {
		return &ToolResult{ForLLM: "buttons must be a non-empty array", IsError: true}
	}
	if len(rawButtons) > 3 {
		return &ToolResult{ForLLM: "WhatsApp supports at most 3 buttons per message", IsError: true}
	}

	type button struct {
		ID    string `json:"id"`
		Title string `json:"title"`
	}
	btns := make([]button, 0, len(rawButtons))
	for _, rb := range rawButtons {
		m, ok := rb.(map[string]any)
		if !ok {
			return &ToolResult{ForLLM: "each button must be an object with id and title", IsError: true}
		}
		id, _ := m["id"].(string)
		title, _ := m["title"].(string)
		if id == "" || title == "" {
			return &ToolResult{ForLLM: "each button must have a non-empty id and title", IsError: true}
		}
		btns = append(btns, button{ID: id, Title: title})
	}

	btnsJSON, err := json.Marshal(btns)
	if err != nil {
		return &ToolResult{ForLLM: fmt.Sprintf("encode buttons: %v", err), IsError: true}
	}

	channel, _ := args["channel"].(string)
	chatID, _ := args["chat_id"].(string)
	if channel == "" {
		channel = ToolChannel(ctx)
	}
	if chatID == "" {
		chatID = ToolChatID(ctx)
	}

	if channel == "" || chatID == "" {
		return &ToolResult{ForLLM: "no target channel/chat — specify channel and chat_id or use from a conversation context", IsError: true}
	}

	if t.sendCallback == nil {
		return &ToolResult{ForLLM: "interactive message sending not configured", IsError: true}
	}

	metadata := map[string]string{
		"wa_interactive_type": "buttons",
		"wa_buttons_json":     string(btnsJSON),
	}
	if err := t.sendCallback(channel, chatID, body, metadata); err != nil {
		return &ToolResult{
			ForLLM:  fmt.Sprintf("send buttons: %v", err),
			IsError: true,
			Err:     err,
		}
	}

	return &ToolResult{
		ForLLM: fmt.Sprintf("Button message sent to %s:%s. Waiting for user to tap a button.", channel, chatID),
		Silent: true,
	}
}

// SendWhatsAppListTool lets the LLM send a WhatsApp list-picker message with
// one or more sections of selectable rows. The user's selection is delivered
// back as a normal inbound message whose Content is the tapped row ID.
type SendWhatsAppListTool struct {
	sendCallback SendInteractiveCallback
}

func NewSendWhatsAppListTool() *SendWhatsAppListTool {
	return &SendWhatsAppListTool{}
}

func (t *SendWhatsAppListTool) SetSendCallback(cb SendInteractiveCallback) {
	t.sendCallback = cb
}

func (t *SendWhatsAppListTool) Name() string {
	return "send_whatsapp_list"
}

func (t *SendWhatsAppListTool) Description() string {
	return "Send a WhatsApp list-picker message with one or more sections of selectable items. " +
		"The user's selection is delivered back as the next inbound message with Content equal to the tapped row id. " +
		"Use this when there are more than 3 choices or when items benefit from optional descriptions."
}

func (t *SendWhatsAppListTool) Parameters() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"body": map[string]any{
				"type":        "string",
				"description": "Message text displayed above the list.",
			},
			"button_label": map[string]any{
				"type":        "string",
				"description": "Label for the button that opens the list (e.g. \"Choose format\").",
			},
			"sections": map[string]any{
				"type":        "array",
				"description": "One or more sections, each with a title and selectable rows.",
				"items": map[string]any{
					"type": "object",
					"properties": map[string]any{
						"title": map[string]any{
							"type":        "string",
							"description": "Section header.",
						},
						"rows": map[string]any{
							"type":        "array",
							"description": "Selectable rows in this section.",
							"items": map[string]any{
								"type": "object",
								"properties": map[string]any{
									"id": map[string]any{
										"type":        "string",
										"description": "Unique row identifier echoed back as inbound Content when selected.",
									},
									"title": map[string]any{
										"type":        "string",
										"description": "Row label shown to the user.",
									},
									"description": map[string]any{
										"type":        "string",
										"description": "Optional sub-text shown below the title.",
									},
								},
								"required": []string{"id", "title"},
							},
						},
					},
					"required": []string{"title", "rows"},
				},
			},
			"channel": map[string]any{
				"type":        "string",
				"description": "Optional: target channel (defaults to current conversation channel).",
			},
			"chat_id": map[string]any{
				"type":        "string",
				"description": "Optional: target chat/user ID (defaults to current chat).",
			},
		},
		"required": []string{"body", "button_label", "sections"},
	}
}

func (t *SendWhatsAppListTool) Execute(ctx context.Context, args map[string]any) *ToolResult {
	body, ok := args["body"].(string)
	if !ok || body == "" {
		return &ToolResult{ForLLM: "body is required", IsError: true}
	}
	buttonLabel, _ := args["button_label"].(string)
	if buttonLabel == "" {
		buttonLabel = "Choose"
	}

	rawSections, ok := args["sections"].([]any)
	if !ok || len(rawSections) == 0 {
		return &ToolResult{ForLLM: "sections must be a non-empty array", IsError: true}
	}

	type row struct {
		ID          string `json:"id"`
		Title       string `json:"title"`
		Description string `json:"description,omitempty"`
	}
	type section struct {
		Title string `json:"title"`
		Rows  []row  `json:"rows"`
	}

	sections := make([]section, 0, len(rawSections))
	for _, rs := range rawSections {
		sm, ok := rs.(map[string]any)
		if !ok {
			return &ToolResult{ForLLM: "each section must be an object with title and rows", IsError: true}
		}
		title, _ := sm["title"].(string)
		rawRows, _ := sm["rows"].([]any)
		if len(rawRows) == 0 {
			return &ToolResult{ForLLM: "each section must have at least one row", IsError: true}
		}
		rows := make([]row, 0, len(rawRows))
		for _, rr := range rawRows {
			rm, ok := rr.(map[string]any)
			if !ok {
				return &ToolResult{ForLLM: "each row must be an object with id and title", IsError: true}
			}
			id, _ := rm["id"].(string)
			rtitle, _ := rm["title"].(string)
			desc, _ := rm["description"].(string)
			if id == "" || rtitle == "" {
				return &ToolResult{ForLLM: "each row must have a non-empty id and title", IsError: true}
			}
			rows = append(rows, row{ID: id, Title: rtitle, Description: desc})
		}
		sections = append(sections, section{Title: title, Rows: rows})
	}

	sectionsJSON, err := json.Marshal(sections)
	if err != nil {
		return &ToolResult{ForLLM: fmt.Sprintf("encode sections: %v", err), IsError: true}
	}

	channel, _ := args["channel"].(string)
	chatID, _ := args["chat_id"].(string)
	if channel == "" {
		channel = ToolChannel(ctx)
	}
	if chatID == "" {
		chatID = ToolChatID(ctx)
	}

	if channel == "" || chatID == "" {
		return &ToolResult{ForLLM: "no target channel/chat — specify channel and chat_id or use from a conversation context", IsError: true}
	}

	if t.sendCallback == nil {
		return &ToolResult{ForLLM: "interactive message sending not configured", IsError: true}
	}

	metadata := map[string]string{
		"wa_interactive_type": "list",
		"wa_list_json":        string(sectionsJSON),
		"wa_list_button":      buttonLabel,
	}
	if err := t.sendCallback(channel, chatID, body, metadata); err != nil {
		return &ToolResult{
			ForLLM:  fmt.Sprintf("send list: %v", err),
			IsError: true,
			Err:     err,
		}
	}

	return &ToolResult{
		ForLLM: fmt.Sprintf("List message sent to %s:%s. Waiting for user to select an item.", channel, chatID),
		Silent: true,
	}
}
