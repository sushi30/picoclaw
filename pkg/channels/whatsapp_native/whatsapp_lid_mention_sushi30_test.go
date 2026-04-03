//go:build whatsapp_native

package whatsapp

// Tests for isMentionedInGroup — the group mention detection helper added to fix
// LID (Linked Identity) session mention handling.
//
// Root cause: WhatsApp LID sessions have two identifiers for the bot device:
//   - phone JID  e.g. 33605951278@s.whatsapp.net  → User="33605951278"
//   - LID        e.g. 24005243363514@lid           → User="24005243363514"
//
// Group @mentions display and transmit the LID user part, not the phone number.
// The old code only checked Store.ID (phone), so all mentions were missed.
// The fix checks both identifiers and adds a text-based fallback for plain
// Conversation messages where ContextInfo is absent.

import (
	"testing"

	"go.mau.fi/whatsmeow/proto/waE2E"
	"google.golang.org/protobuf/proto"
)

const (
	testPhoneUser = "33605951278"
	testLIDUser   = "24005243363514"
)

var testBotUsers = []string{testPhoneUser, testLIDUser}

func extMsg(text string, mentionedJIDs []string) *waE2E.Message {
	return &waE2E.Message{
		ExtendedTextMessage: &waE2E.ExtendedTextMessage{
			Text: proto.String(text),
			ContextInfo: &waE2E.ContextInfo{
				MentionedJID: mentionedJIDs,
			},
		},
	}
}

func convMsg(text string) *waE2E.Message {
	return &waE2E.Message{Conversation: proto.String(text)}
}

func TestIsMentionedInGroup(t *testing.T) {
	tests := []struct {
		name    string
		msg     *waE2E.Message
		content string
		users   []string
		want    bool
	}{
		{
			name:    "LID in ContextInfo MentionedJID",
			msg:     extMsg("@"+testLIDUser+" hello", []string{testLIDUser + "@lid"}),
			content: "@" + testLIDUser + " hello",
			users:   testBotUsers,
			want:    true,
		},
		{
			name:    "phone JID in ContextInfo MentionedJID",
			msg:     extMsg("hello @"+testPhoneUser, []string{testPhoneUser + "@s.whatsapp.net"}),
			content: "hello @" + testPhoneUser,
			users:   testBotUsers,
			want:    true,
		},
		{
			name:    "LID text fallback — plain Conversation message",
			msg:     convMsg("@" + testLIDUser + " what is 1+1?"),
			content: "@" + testLIDUser + " what is 1+1?",
			users:   testBotUsers,
			want:    true,
		},
		{
			name:    "phone text fallback — plain Conversation message",
			msg:     convMsg("@" + testPhoneUser + " help"),
			content: "@" + testPhoneUser + " help",
			users:   testBotUsers,
			want:    true,
		},
		{
			name:    "no mention in plain message",
			msg:     convMsg("just a group message"),
			content: "just a group message",
			users:   testBotUsers,
			want:    false,
		},
		{
			name:    "ContextInfo present but empty MentionedJID and no text match",
			msg:     extMsg("hello everyone", []string{}),
			content: "hello everyone",
			users:   testBotUsers,
			want:    false,
		},
		{
			name:    "mention of different user — no match",
			msg:     extMsg("@99999999 hi", []string{"99999999@s.whatsapp.net"}),
			content: "@99999999 hi",
			users:   testBotUsers,
			want:    false,
		},
		{
			name:    "empty botUsers — never matches",
			msg:     extMsg("@"+testLIDUser+" hi", []string{testLIDUser + "@lid"}),
			content: "@" + testLIDUser + " hi",
			users:   nil,
			want:    false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := isMentionedInGroup(tc.msg, tc.content, tc.users)
			if got != tc.want {
				t.Errorf("isMentionedInGroup() = %v, want %v", got, tc.want)
			}
		})
	}
}
