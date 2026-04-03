package commands

import "context"

func debugCommand() Definition {
	return Definition{
		Name:        "debug",
		Description: "Toggle per-chat debug messages showing tool calls",
		SubCommands: []SubCommand{
			{
				Name:        "on",
				Description: "Enable debug tool-call messages for this chat",
				Handler: func(_ context.Context, req Request, rt *Runtime) error {
					if rt == nil || rt.SetDebugMode == nil {
						return req.Reply(unavailableMsg)
					}
					rt.SetDebugMode(true)
					return req.Reply("Debug mode enabled. Tool calls will be shown in this chat.")
				},
			},
			{
				Name:        "off",
				Description: "Disable debug tool-call messages for this chat",
				Handler: func(_ context.Context, req Request, rt *Runtime) error {
					if rt == nil || rt.SetDebugMode == nil {
						return req.Reply(unavailableMsg)
					}
					rt.SetDebugMode(false)
					return req.Reply("Debug mode disabled.")
				},
			},
		},
	}
}
