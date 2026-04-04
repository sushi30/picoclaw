package channels

import (
	"context"

	"github.com/sipeed/picoclaw/pkg/bus"
	"github.com/sipeed/picoclaw/pkg/commands"
)

// TypingCapable — channels that can show a typing/thinking indicator.
// StartTyping begins the indicator and returns a stop function.
// The stop function MUST be idempotent and safe to call multiple times.
type TypingCapable interface {
	StartTyping(ctx context.Context, chatID string) (stop func(), err error)
}

// MessageEditor — channels that can edit an existing message.
// messageID is always string; channels convert platform-specific types internally.
type MessageEditor interface {
	EditMessage(ctx context.Context, chatID string, messageID string, content string) error
}

// MessageDeleter — channels that can delete a message by ID.
type MessageDeleter interface {
	DeleteMessage(ctx context.Context, chatID string, messageID string) error
}

// ReactionCapable — channels that can add a reaction (e.g. 👀) to an inbound message.
// ReactToMessage adds a reaction and returns an undo function to remove it.
// The undo function MUST be idempotent and safe to call multiple times.
type ReactionCapable interface {
	ReactToMessage(ctx context.Context, chatID, messageID string) (undo func(), err error)
}

// PlaceholderCapable — channels that can send a placeholder message
// (e.g. "Thinking... 💭") that will later be edited to the actual response.
// The channel MUST also implement MessageEditor for the placeholder to be useful.
// SendPlaceholder returns the platform message ID of the placeholder so that
// Manager.preSend can later edit it via MessageEditor.EditMessage.
type PlaceholderCapable interface {
	SendPlaceholder(ctx context.Context, chatID string) (messageID string, err error)
}

// StreamingCapable — channels that can show partial LLM output in real-time.
// The channel SHOULD gracefully degrade if the platform rejects streaming
// (e.g. Telegram bot without forum mode). In that case, Update becomes a no-op
// and Finalize still delivers the final message.
type StreamingCapable interface {
	BeginStream(ctx context.Context, chatID string) (Streamer, error)
}

// Streamer is defined in pkg/bus to avoid circular imports.
// This alias keeps channel implementations using channels.Streamer unchanged.
type Streamer = bus.Streamer

// PlaceholderRecorder is injected into channels by Manager.
// Channels call these methods on inbound to register typing/placeholder state.
// Manager uses the registered state on outbound to stop typing and edit placeholders.
type PlaceholderRecorder interface {
	RecordPlaceholder(channel, chatID, placeholderID string)
	RecordTypingStop(channel, chatID string, stop func())
	RecordReactionUndo(channel, chatID string, undo func())
}

// CommandRegistrarCapable is implemented by channels that can register
// command menus with their upstream platform (e.g. Telegram BotCommand).
// Channels that do not support platform-level command menus can ignore it.
type CommandRegistrarCapable interface {
	RegisterCommands(ctx context.Context, defs []commands.Definition) error
}

// InteractiveCapable — channels that can send interactive messages with
// user-selectable options. Implemented by channels that support native
// widget types such as button quick-replies and list pickers.
//
// When the user selects an option the channel emits a normal InboundMessage
// whose Content is the selected button/row ID and whose Metadata contains:
//   - "wa_interactive_type":        "button" | "list"
//   - "wa_interactive_selected_id": the ID that was tapped
//   - "wa_interactive_display_text": human-readable label the user tapped
type InteractiveCapable interface {
	SendButtons(ctx context.Context, chatID, body string, buttons []InteractiveButton) error
	SendList(ctx context.Context, chatID, body, buttonLabel string, sections []InteractiveSection) error
}

// InteractiveButton is a single quick-reply button. ID is echoed back as the
// inbound Content when the user taps; Title is the label displayed on screen.
// WhatsApp limits buttons to 3 per message and IDs to ~20 characters.
type InteractiveButton struct {
	ID    string
	Title string
}

// InteractiveSection groups a set of selectable rows in a list message.
type InteractiveSection struct {
	Title string
	Rows  []InteractiveRow
}

// InteractiveRow is a single selectable item inside an InteractiveSection.
type InteractiveRow struct {
	ID          string
	Title       string
	Description string // optional sub-text shown below the title
}
