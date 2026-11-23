package antigravity

import (
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/seanthegeek/doubleagent/internal/input"
	"github.com/seanthegeek/doubleagent/internal/jsonvalue"
	"github.com/seanthegeek/doubleagent/internal/parserkit"
)

// argKeys are the JSON arguments that best identify a proposed tool call,
// tried in order.
func argKeys() []string {
	return []string{"CommandLine", "Command", "AbsolutePath", "TargetFile", "AbsolutePathUri", "Path",
		"DirectoryPath", "Url", "Query", "SearchPath", "Pattern", "Prompt", "Instruction", "TaskName", "Name"}
}

// toolArgsSummary returns the argument that names a tool call's action,
// else the arguments as JSON; text that is not JSON is returned as it is.
// The arguments' repeated member names are noted in the parse of artifact.
func toolArgsSummary(artifact *input.Artifact, argumentsJSON string) string {
	args := jsonvalue.NewObject()
	if argumentsJSON != "" {
		decoded, err := parserkit.DecodeJSON(artifact, argumentsJSON)
		if err != nil {
			return argumentsJSON
		}
		args = decoded
	}
	for _, key := range argKeys() {
		value := args.Get(key)
		empty := !parserkit.Present(value) ||
			((value.IsArray() || value.IsObject()) && value.Len() == 0)
		if empty {
			continue
		}
		if value.IsString() {
			return value.StringValue()
		}
		return parserkit.JSONText(value)
	}
	return parserkit.JSONText(args)
}

// pathOf returns the path of a PathScopeItem.
func pathOf(scope message) string {
	return uriPath(scope.first("absolute_uri", "absolute_path_migrate_me_to_uri"))
}

// output returns a command step's output: the full or cut combined output,
// else the combined, stdout, stderr or output text.
func output(payload message) string {
	if text := payload.child("combined_output").first("full", "truncated"); text != "" {
		return text
	}
	return payload.first("combined", "stdout", "stderr", "output")
}

// flag returns text when the field is true, else "".
func flag(payload message, name, text string) string {
	if payload.set(name) {
		return text
	}
	return ""
}

// exitText returns "exit=<code>" when the payload has an exit code.
func exitText(payload message) string {
	if payload.has("exit_code") {
		return "exit=" + payload.text("exit_code")
	}
	return ""
}

// describers returns, by step kind, the function that describes a step's
// result or event in one line.
func describers() map[string]func(message) string {
	return map[string]func(message) string{
		"run_command": runCommandText, "command_status": commandText, "shell_exec": commandText,
		"send_command_input": func(payload message) string {
			return "input to " + payload.text("command_id") + ": " + payload.text("input") +
				flag(payload, "terminate", " [terminate]")
		},
		"view_file": viewFileText,
		"list_directory": func(payload message) string {
			return uriPath(payload.text("directory_path_uri"))
		},
		"grep_search": func(payload message) string {
			return parserkit.JoinNonEmpty(" | ", payload.text("query"), uriPath(payload.text("search_path_uri")), payload.text("command_run"))
		},
		"find": func(payload message) string {
			return parserkit.JoinNonEmpty(" | ", payload.text("pattern"), payload.text("search_directory"), payload.text("command_run"))
		},
		"write_to_file": func(payload message) string {
			return uriPath(payload.text("target_file_uri")) + flag(payload, "file_created", " [created]")
		},
		"file_change": func(payload message) string {
			return parserkit.JoinNonEmpty(" | ", uriPath(payload.text("absolute_path_uri")), payload.text("instruction"))
		},
		"code_action":      codeActionText,
		"read_url_content": func(payload message) string { return payload.first("resolved_url", "url") },
		"search_web":       func(payload message) string { return payload.text("query") },
		"mcp_tool":         mcpToolText,
		"agency_tool_call": func(payload message) string {
			return payload.text("agent_name") + "." + payload.text("function_name")
		},
		"invoke_subagent": subagentText,
		"ask_question":    askQuestionText,
		"generic":         func(payload message) string { return payload.child("result").text("result") },
	}
}

// eventDescribers returns, by step kind, the function that describes a
// step that is an event rather than a tool's result.
func eventDescribers() map[string]func(message) string {
	return map[string]func(message) string{
		"checkpoint": func(payload message) string {
			return "checkpoint: " + payload.first("conversation_title", "session_summary", "user_intent")
		},
		"system_message": func(payload message) string {
			event := payload.first("event_type")
			if event == "" {
				event = "system_message"
			}
			return event + ": " + payload.text("message")
		},
		"ephemeral_message": func(payload message) string { return "ephemeral: " + payload.text("content") },
		"task_boundary": func(payload message) string {
			return "task " + payload.text("task_name") + " [" + payload.text("task_status") + "]: " +
				payload.text("task_summary")
		},
		"notify_user":   func(payload message) string { return "notify user: " + payload.text("notification_content") },
		"error":         errorText,
		"error_message": func(payload message) string { return errorText(payload.child("error")) },
		"finish":        func(payload message) string { return "finish: " + payload.text("output_string") },
		"memory": func(payload message) string {
			return "memory " + payload.text("memory_id") + " action=" + payload.text("action")
		},
		"directory_rules": func(payload message) string {
			var paths []string
			for _, uri := range payload.list("rule_file_uris") {
				paths = append(paths, uriPath(valueText(uri)))
			}
			return "directory rules: " + strings.Join(paths, " ")
		},
		"conversation_history": func(payload message) string {
			return "conversation history injected (" +
				strconv.Itoa(utf8.RuneCountInString(payload.text("content"))) + " chars)"
		},
	}
}

// describePayload returns one line describing a tool step's result or a
// system event.
func describePayload(kind string, payload message) string {
	if describe, ok := describers()[kind]; ok {
		return describe(payload)
	}
	if describe, ok := eventDescribers()[kind]; ok {
		return describe(payload)
	}
	return parserkit.JSONText(jsonValue(payload.decoded))
}

// runCommandText describes a command run.
func runCommandText(payload message) string {
	return parserkit.JoinNonEmpty(" | ", payload.first("command_line", "command"), flag(payload, "user_rejected", "[rejected by user]"),
		flag(payload, "timed_out", "[timed out]"), exitText(payload), output(payload))
}

// mcpToolText describes an MCP tool call.
func mcpToolText(payload message) string {
	return parserkit.JoinNonEmpty(" | ", payload.text("server_name")+"/"+payload.child("tool_call").text("name"),
		flag(payload, "user_rejected", "[rejected by user]"), payload.text("result_string"))
}

// commandText describes a command status or shell step.
func commandText(payload message) string {
	out := output(payload)
	if out == "" {
		out = payload.text("error_message")
	}
	return parserkit.JoinNonEmpty(" | ", payload.text("command"), exitText(payload), out)
}

// viewFileText describes a file view, with its line range when it has
// one.
func viewFileText(payload message) string {
	lineRange := ""
	if payload.set("start_line") || payload.set("end_line") {
		lineRange = ":" + payload.text("start_line") + "-" + payload.text("end_line")
	}
	return uriPath(payload.text("absolute_path_uri")) + lineRange
}

// codeActionText describes a code action: its kind, its path and its
// description.
func codeActionText(payload message) string {
	spec := payload.child("action_spec")
	actionKind, path := "", ""
	for _, key := range []string{"command", "create_file", "delete_file", "sed"} {
		if action := spec.child(key); !action.empty() {
			scope := action.child("file")
			if scope.empty() {
				scope = action.child("path")
			}
			actionKind, path = key, pathOf(scope)
			break
		}
	}
	description := payload.first("description")
	if description == "" {
		description = payload.text("code_instruction")
	}
	return parserkit.JoinNonEmpty(" | ", actionKind, path, description)
}

// subagentText describes a subagent call and the conversations it began.
func subagentText(payload message) string {
	var children []string
	for _, result := range payload.children("results") {
		if result.set("conversation_id") {
			children = append(children, result.text("conversation_id"))
		}
	}
	listed := ""
	if len(children) > 0 {
		listed = "children: " + strings.Join(children, " ")
	}
	return parserkit.JoinNonEmpty(" | ", payload.text("subagent_name"), payload.text("prompt"), listed)
}

// askQuestionText describes the questions asked and their answers.
func askQuestionText(payload message) string {
	var answers []string
	for _, question := range payload.children("questions") {
		answer := question.first("write_in_response")
		if answer == "" {
			var options []string
			for _, option := range question.list("selected_option_ids") {
				options = append(options, valueText(option))
			}
			answer = strings.Join(options, " ")
		}
		if answer == "" {
			answer = flag(question, "skipped", "skipped")
		}
		text := question.text("question")
		if answer != "" {
			text += " -> " + answer
		}
		answers = append(answers, text)
	}
	return strings.Join(answers, "; ")
}

// errorText describes error details.
func errorText(details message) string {
	return "error: " + details.first("short_error", "user_error_message", "full_error")
}

// permissionTexts returns one "permission grant:" text per entry of a
// TrajectoryPermissions.
func permissionTexts(permissions message) []string {
	scopes := permissionScopes()
	scopeName := func(entry message) string {
		scope, ok := entry.intValue("scope")
		if !entry.has("scope") {
			scope, ok = 0, true
		}
		if name, known := scopes[scope]; ok && known {
			return name
		}
		return entry.text("scope")
	}
	var out []string
	for _, grant := range permissions.children("permission_grants") {
		resource := grant.child("resource")
		action, target := grant.first("action"), grant.first("target")
		if action == "" {
			action = resource.text("action")
		}
		if target == "" {
			target = resource.text("target")
		}
		text := "permission grant: action=" + action + " target=" + target + " allow=" + boolText(grant, "allow") +
			" scope=" + scopeName(grant)
		if grant.set("user_deny_instruction") {
			text += " deny_instruction=" + grant.text("user_deny_instruction")
		}
		out = append(out, text)
	}
	for _, access := range permissions.children("file_access_permissions") {
		text := "permission grant: path=" + uriPath(access.text("path")) + " is_directory=" +
			boolText(access, "is_directory") + " allow=" + boolText(access, "allow") + " scope=" + scopeName(access)
		if access.set("from_current_step") {
			text += " from_current_step=true"
		}
		out = append(out, text)
	}
	return out
}
