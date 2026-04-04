//go:build whatsapp_native

// PicoClaw - Ultra-lightweight personal AI agent
// License: MIT
//
// Copyright (c) 2026 PicoClaw contributors

package whatsapp

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"github.com/sipeed/picoclaw/pkg/agent"
	"github.com/sipeed/picoclaw/pkg/channels"
)

const approvalButtonPrefix = "wa_approval_"

// WhatsAppApprover implements agent.ToolApprover by sending an interactive
// button message to the user and blocking until they tap Approve or Deny.
//
// To use it:
//  1. Create one with NewWhatsAppApprover(channel).
//  2. Mount it on the hook manager: hm.Mount(agent.NamedHook("wa_approver", approver)).
//  3. Pass it to channel.RegisterApprover(approver) so handleInteractiveResponse
//     can notify it when a matching button ID arrives.
type WhatsAppApprover struct {
	ch      *WhatsAppNativeChannel
	pending sync.Map // reqID (string) → chan bool
}

// NewWhatsAppApprover creates a WhatsAppApprover for the given channel.
func NewWhatsAppApprover(ch *WhatsAppNativeChannel) *WhatsAppApprover {
	return &WhatsAppApprover{ch: ch}
}

// ApproveTool implements agent.ToolApprover. For whatsapp_native conversations
// it sends a two-button message and blocks until the user taps Approve or Deny
// (or the context times out). For other channels it passes through as approved.
func (a *WhatsAppApprover) ApproveTool(ctx context.Context, req *agent.ToolApprovalRequest) (agent.ApprovalDecision, error) {
	if req.Channel != "whatsapp_native" {
		return agent.ApprovalDecision{Approved: true}, nil
	}

	// Use TurnID as a correlation key — it is unique per agent turn.
	reqID := req.Meta.TurnID
	if reqID == "" {
		return agent.ApprovalDecision{Approved: false, Reason: "missing turn ID for approval correlation"}, nil
	}

	body := fmt.Sprintf("Tool approval required\n\nTool: %s", req.Tool)
	btns := []channels.InteractiveButton{
		{ID: approvalButtonPrefix + "yes_" + reqID, Title: "Approve"},
		{ID: approvalButtonPrefix + "no_" + reqID, Title: "Deny"},
	}

	responseCh := make(chan bool, 1)
	a.pending.Store(reqID, responseCh)
	defer a.pending.Delete(reqID)

	if err := a.ch.SendButtons(ctx, req.ChatID, body, btns); err != nil {
		return agent.ApprovalDecision{}, fmt.Errorf("send approval buttons: %w", err)
	}

	select {
	case approved := <-responseCh:
		reason := ""
		if !approved {
			reason = "denied by user via WhatsApp"
		}
		return agent.ApprovalDecision{Approved: approved, Reason: reason}, nil
	case <-ctx.Done():
		return agent.ApprovalDecision{Approved: false, Reason: "approval timed out"}, nil
	}
}

// Notify is called by handleInteractiveResponse when a button tap arrives.
// If the button ID corresponds to a pending approval request, the waiting
// ApproveTool call is unblocked. Other button IDs are silently ignored.
func (a *WhatsAppApprover) Notify(buttonID string) {
	if !strings.HasPrefix(buttonID, approvalButtonPrefix) {
		return
	}
	rest := strings.TrimPrefix(buttonID, approvalButtonPrefix)

	var approved bool
	var reqID string
	switch {
	case strings.HasPrefix(rest, "yes_"):
		approved = true
		reqID = strings.TrimPrefix(rest, "yes_")
	case strings.HasPrefix(rest, "no_"):
		reqID = strings.TrimPrefix(rest, "no_")
	default:
		return
	}

	if v, ok := a.pending.Load(reqID); ok {
		ch := v.(chan bool)
		select {
		case ch <- approved:
		default:
		}
	}
}
