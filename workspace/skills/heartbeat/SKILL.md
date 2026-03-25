---
name: heartbeat
description: Manage recurring background tasks in HEARTBEAT.md — add, remove, or list scheduled tasks the agent runs automatically at a fixed interval.
metadata: {"nanobot":{"emoji":"💓"}}
---

# Heartbeat Task Manager

Manage the agent's periodic task list. The heartbeat service reads `HEARTBEAT.md` from the workspace root and runs all listed tasks on a fixed interval (default: every 30 minutes).

## When to use (trigger phrases)

Use this skill when the user asks to:
- "run X every hour / periodically / on a schedule"
- "remind me every day to …"
- "check X regularly / in the background"
- "add a heartbeat task"
- "remove / stop / delete a heartbeat task"
- "show / list my heartbeat tasks"
- "what's in my heartbeat?"

## File format

Tasks live in `HEARTBEAT.md` in the workspace root, **below** the marker line:

```
Add your heartbeat tasks below this line:
```

Each task is a plain line or bullet. Example:

```
Add your heartbeat tasks below this line:
- Check the weather and send a summary
- Summarize any unread Telegram messages
- If BTC price dropped more than 5% since last check, notify me
```

Tasks that start with `#` (comments) or are blank are ignored.

## Operations

### List current tasks

Use `read_file` on `HEARTBEAT.md`, then show the lines below the marker:

```
tool: read_file
path: {workspace}/HEARTBEAT.md
```

Present only the lines after `Add your heartbeat tasks below this line:`, skipping blank lines and `#` comments.

### Add a task

Use `append_file` to add a new task line at the end of the file:

```
tool: append_file
path: {workspace}/HEARTBEAT.md
content: "\n- <task description>"
```

If the file doesn't exist yet, create it from scratch first (see below).

### Remove a task

Use `edit_file` to replace the matching task line with nothing. The `old_text` must match exactly including the leading `- ` and trailing newline:

```
tool: edit_file
path: {workspace}/HEARTBEAT.md
old_text: "- <exact task line>\n"
new_text: ""
```

If unsure of the exact wording, read the file first and copy the line verbatim.

### Create HEARTBEAT.md from scratch (if missing)

Use `write_file` with the standard template:

```
tool: write_file
path: {workspace}/HEARTBEAT.md
overwrite: false
content: |
  # Heartbeat Check List

  ## Instructions

  - Execute ALL tasks listed below. Do NOT skip any task.
  - For simple tasks (e.g., report current time), respond directly.
  - For complex tasks that may take time, use the spawn tool to create a subagent.
  - After spawning a subagent, CONTINUE to process remaining tasks.
  - Only respond with HEARTBEAT_OK when ALL tasks are done AND nothing needs attention.

  ---

  Add your heartbeat tasks below this line:
```

Then append the new task with `append_file`.

## Rules

- Always confirm with the user what was added/removed after editing the file.
- After removing a task, read the file again to verify it is gone.
- Do not rewrite unrelated parts of the file — only append or selectively remove task lines.
- If the user asks to "stop" a scheduled task, remove it from HEARTBEAT.md (and also check `cron list` in case it was added via the cron tool instead).
- The heartbeat interval is set in config (`heartbeat.interval`, minutes). Editing the interval requires changing the config, not this file.
