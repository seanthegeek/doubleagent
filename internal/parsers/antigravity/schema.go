package antigravity

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/seanthegeek/doubleagent/internal/protowire"
)

// schemaText lists the protobuf fields read, one per line as "<message>
// <number> <name> <kind>", each message's fields in the order the
// descriptors embedded in the agy binary declare them; only the fields the
// parser reads are listed. A name starting with "+" is a repeated field. The
// order of Step's fields decides which step kind a step is (payloadKinds).
const schemaText = `
Step 1 type int
Step 4 status int
Step 5 metadata CortexStepMetadata
Step 10 code_action CortexStepCodeAction
Step 12 finish CortexStepFinish
Step 13 grep_search CortexStepGrepSearch
Step 14 view_file CortexStepViewFile
Step 15 list_directory CortexStepListDirectory
Step 19 user_input CortexStepUserInput
Step 20 planner_response CortexStepPlannerResponse
Step 23 write_to_file CortexStepWriteToFile
Step 24 error_message CortexStepErrorMessage
Step 28 run_command CortexStepRunCommand
Step 30 checkpoint CortexStepCheckpoint
Step 31 error CortexErrorDetails
Step 34 find CortexStepFind
Step 37 command_status CortexStepCommandStatus
Step 38 memory CortexStepMemory
Step 40 read_url_content CortexStepReadUrlContent
Step 42 search_web CortexStepSearchWeb
Step 47 mcp_tool CortexStepMcpTool
Step 93 task_boundary CortexStepTaskBoundary
Step 94 notify_user CortexStepNotifyUser
Step 98 file_change CortexStepFileChange
Step 103 ephemeral_message CortexStepEphemeralMessage
Step 111 conversation_history CortexStepConversationHistory
Step 113 send_command_input CortexStepSendCommandInput
Step 114 system_message CortexStepSystemMessage
Step 116 agency_tool_call CortexStepAgencyToolCall
Step 127 shell_exec CortexStepShellExec
Step 133 permissions TrajectoryPermissions
Step 140 generic CortexStepGeneric
Step 143 invoke_subagent CortexStepInvokeSubagent
Step 154 ask_question CortexStepAskQuestion
Step 156 directory_rules CortexStepDirectoryRules
CortexStepMetadata 1 created_at ts
CortexStepMetadata 3 source int
CortexStepMetadata 4 tool_call ChatToolCall
CortexStepMetadata 7 finished_generating_at ts
CortexStepMetadata 8 completed_at ts
CortexStepMetadata 12 execution_id str
CortexStepMetadata 30 tool_summary str
CortexStepMetadata 31 tool_action str
CortexStepMetadata 32 started_at ts
ChatToolCall 1 id str
ChatToolCall 2 name str
ChatToolCall 3 arguments_json str
ChatToolCall 9 original_name str
CortexStepUserInput 1 query str
CortexStepUserInput 2 user_response str
CortexStepUserInput 6 is_queued_message bool
CortexStepPlannerResponse 1 response str
CortexStepPlannerResponse 3 thinking str
CortexStepPlannerResponse 6 message_id str
CortexStepPlannerResponse 7 +tool_calls ChatToolCall
CortexStepPlannerResponse 8 modified_response str
CortexStepPlannerResponse 16 raw_thinking str
CortexStepGeneric 1 +args KV
CortexStepGeneric 2 result GenericStepResult
KV 1 key str
KV 2 value str
GenericStepResult 1 result str
GenericStepResult 7 full_output_uri str
CortexStepRunCommand 1 command str
CortexStepRunCommand 2 cwd str
CortexStepRunCommand 4 stdout str
CortexStepRunCommand 5 stderr str
CortexStepRunCommand 6 exit_code int
CortexStepRunCommand 14 user_rejected bool
CortexStepRunCommand 21 combined_output RunCommandOutput
CortexStepRunCommand 23 command_line str
CortexStepRunCommand 34 ran_in_sandbox bool
CortexStepRunCommand 37 timed_out bool
RunCommandOutput 1 full str
RunCommandOutput 2 truncated str
CortexStepCommandStatus 1 command_id str
CortexStepCommandStatus 3 stdout str
CortexStepCommandStatus 4 stderr str
CortexStepCommandStatus 5 exit_code int
CortexStepCommandStatus 9 combined str
CortexStepShellExec 1 command str
CortexStepShellExec 2 exit_code int
CortexStepShellExec 3 output str
CortexStepShellExec 4 error_message str
CortexStepSendCommandInput 1 command_id str
CortexStepSendCommandInput 2 input str
CortexStepSendCommandInput 6 terminate bool
CortexStepViewFile 1 absolute_path_uri str
CortexStepViewFile 2 start_line int
CortexStepViewFile 3 end_line int
CortexStepViewFile 17 is_skill_file bool
CortexStepListDirectory 1 directory_path_uri str
CortexStepGrepSearch 1 query str
CortexStepGrepSearch 10 command_run str
CortexStepGrepSearch 11 search_path_uri str
CortexStepFind 1 pattern str
CortexStepFind 9 command_run str
CortexStepFind 10 search_directory str
CortexStepWriteToFile 1 target_file_uri str
CortexStepWriteToFile 4 file_created bool
CortexStepFileChange 1 absolute_path_uri str
CortexStepFileChange 2 file_change_type int
CortexStepFileChange 5 instruction str
CortexStepCodeAction 1 action_spec ActionSpec
CortexStepCodeAction 8 code_instruction str
CortexStepCodeAction 26 description str
ActionSpec 1 command ActionSpecCommand
ActionSpec 2 create_file ActionSpecPath
ActionSpec 4 delete_file ActionSpecDeleteFile
ActionSpec 5 sed ActionSpecSed
ActionSpecCommand 1 instruction str
ActionSpecCommand 4 file PathScopeItem
ActionSpecPath 1 instruction str
ActionSpecPath 2 path PathScopeItem
ActionSpecDeleteFile 1 path PathScopeItem
ActionSpecSed 3 path PathScopeItem
PathScopeItem 1 absolute_path_migrate_me_to_uri str
PathScopeItem 5 absolute_uri str
CortexStepReadUrlContent 1 url str
CortexStepReadUrlContent 3 resolved_url str
CortexStepSearchWeb 1 query str
CortexStepSearchWeb 5 summary str
CortexStepMcpTool 1 server_name str
CortexStepMcpTool 2 tool_call ChatToolCall
CortexStepMcpTool 3 result_string str
CortexStepMcpTool 7 user_rejected bool
CortexStepAgencyToolCall 1 agent_name str
CortexStepAgencyToolCall 2 function_name str
CortexStepInvokeSubagent 1 subagent_name str
CortexStepInvokeSubagent 2 prompt str
CortexStepInvokeSubagent 5 conversation_id str
CortexStepInvokeSubagent 10 +results SubagentResult
SubagentResult 1 conversation_id str
CortexStepAskQuestion 1 +questions AskQuestionEntry
AskQuestionEntry 1 question str
AskQuestionEntry 4 +selected_option_ids str
AskQuestionEntry 5 write_in_response str
AskQuestionEntry 6 skipped bool
CortexStepCheckpoint 4 user_intent str
CortexStepCheckpoint 5 session_summary str
CortexStepCheckpoint 10 conversation_title str
CortexStepMemory 1 memory_id str
CortexStepMemory 3 action int
CortexStepFinish 2 output_string str
CortexStepErrorMessage 3 error CortexErrorDetails
CortexErrorDetails 1 user_error_message str
CortexErrorDetails 2 short_error str
CortexErrorDetails 3 full_error str
CortexStepSystemMessage 1 message str
CortexStepSystemMessage 3 event_type str
CortexStepEphemeralMessage 1 content str
CortexStepTaskBoundary 1 task_name str
CortexStepTaskBoundary 2 task_status str
CortexStepTaskBoundary 3 task_summary str
CortexStepNotifyUser 2 notification_content str
CortexStepDirectoryRules 2 +rule_file_uris str
CortexStepConversationHistory 1 content str
TrajectoryPermissions 1 +file_access_permissions FileAccessPermission
TrajectoryPermissions 2 +permission_grants PermissionGrant
PermissionGrant 1 resource PermissionResource
PermissionGrant 2 allow bool
PermissionGrant 3 scope int
PermissionGrant 4 user_deny_instruction str
PermissionGrant 5 action str
PermissionGrant 6 target str
PermissionResource 1 action str
PermissionResource 2 target str
FileAccessPermission 1 path str
FileAccessPermission 2 is_directory bool
FileAccessPermission 3 allow bool
FileAccessPermission 4 scope int
FileAccessPermission 5 from_current_step bool
CortexTrajectoryMetadata 1 +workspaces CortexWorkspaceMetadata
CortexTrajectoryMetadata 2 created_at ts
CortexTrajectoryMetadata 5 parent_conversation_id str
CortexTrajectoryMetadata 6 root_conversation_id str
CortexTrajectoryMetadata 7 +workspace_uris str
CortexTrajectoryMetadata 17 nesting_depth int
CortexTrajectoryMetadata 18 project_id str
CortexWorkspaceMetadata 1 workspace_folder_absolute_uri str
CortexWorkspaceMetadata 2 git_root_absolute_uri str
CortexWorkspaceMetadata 4 branch_name str
CortexStepGeneratorMetadata 1 chat_model ChatModelMetadata
CortexStepGeneratorMetadata 2 +step_indices int
ChatModelMetadata 19 response_model str
ChatModelMetadata 21 model_display_name str
ChatModelMetadata 22 response_model_full str`

// newSchema returns the schema schemaText lists.
func newSchema() (*protowire.Schema, error) {
	messages := map[string][]protowire.Field{}
	for line := range strings.Lines(schemaText) {
		words := strings.Fields(line)
		if len(words) == 0 {
			continue
		}
		if len(words) != 4 {
			return nil, fmt.Errorf("antigravity schema: line %q", line)
		}
		number, err := strconv.ParseUint(words[1], 10, 64)
		if err != nil {
			return nil, fmt.Errorf("antigravity schema: line %q: %w", line, err)
		}
		messages[words[0]] = append(messages[words[0]], protowire.Field{Number: number, Name: words[2], Kind: words[3]})
	}
	return protowire.NewSchema(messages)
}

// permissionScopes are exa.cortex_pb.PermissionScope, lower-cased without
// the PERMISSION_SCOPE_ prefix: how long a permission grant holds.
func permissionScopes() map[int64]string {
	return map[int64]string{
		0: "unspecified", 1: "once", 2: "conversation", 3: "workspace",
		4: "global", 5: "project",
	}
}

// trajectorySources are exa.cortex_pb.CortexTrajectorySource, lower-cased
// without the prefix: the surface that started the conversation.
func trajectorySources() map[int64]string {
	return map[int64]string{
		0: "unspecified", 1: "cascade_client", 2: "explain_problem", 3: "refactor_function",
		4: "eval", 5: "eval_task", 6: "async_prr", 7: "async_cf",
		8: "async_sl", 9: "async_prd", 10: "async_cm", 12: "interactive_cascade",
		13: "replay", 15: "sdk", 16: "subagent", 17: "cli",
		18: "jetbox", 19: "agent_api", 20: "cider", 21: "gemini_app",
		22: "vscode", 23: "visual_studio",
	}
}

// stepTypes are exa.cortex_pb.CortexStepType, lower-cased without the
// prefix.
func stepTypes() map[int64]string {
	return map[int64]string{
		0: "unspecified", 1: "dummy", 2: "finish", 3: "plan_input",
		4: "mquery", 5: "code_action", 6: "git_commit", 7: "grep_search",
		8: "view_file", 9: "list_directory", 10: "compile", 13: "view_code_item",
		14: "user_input", 15: "planner_response", 17: "error_message", 21: "run_command",
		23: "checkpoint", 24: "propose_code", 25: "find", 27: "suggested_responses",
		28: "command_status", 29: "memory", 31: "read_url_content", 32: "view_content_chunk",
		33: "search_web", 34: "retrieve_memory", 38: "mcp_tool", 39: "manager_feedback",
		40: "tool_call_proposal", 41: "tool_call_choice", 42: "trajectory_choice", 45: "clipboard",
		47: "view_file_outline", 49: "post_pr_review", 51: "list_resources", 52: "read_resource",
		53: "lint_diff", 54: "find_all_references", 55: "brain_update", 56: "open_browser_url",
		57: "run_extension_code", 59: "proposal_feedback", 60: "trajectory_search", 61: "execute_browser_javascript",
		62: "list_browser_pages", 63: "capture_browser_screenshot", 64: "click_browser_pixel", 65: "read_terminal",
		66: "capture_browser_console_logs", 67: "read_browser_page", 68: "browser_get_dom", 73: "code_search",
		74: "browser_input", 75: "browser_move_mouse", 76: "browser_select_option", 77: "browser_scroll_up",
		78: "browser_scroll_down", 79: "browser_click_element", 80: "browser_press_key", 81: "task_boundary",
		82: "notify_user", 83: "code_acknowledgement", 84: "internal_search", 85: "browser_subagent",
		86: "file_change", 87: "move", 88: "browser_scroll", 89: "knowledge_generation",
		90: "ephemeral_message", 91: "generate_image", 92: "delete_directory", 93: "compile_applet",
		94: "install_applet_dependencies", 95: "install_applet_package", 96: "browser_resize_window", 97: "browser_drag_pixel_to_pixel",
		98: "conversation_history", 99: "knowledge_artifacts", 100: "send_command_input", 101: "system_message",
		102: "wait", 103: "agency_tool_call", 104: "cider_agent_dummy", 105: "build_cleaner",
		106: "blaze_build_targets", 107: "blaze_test_targets", 108: "set_up_firebase", 109: "moma",
		110: "restart_dev_server", 111: "deploy_firebase", 112: "shell_exec", 113: "browser_mouse_wheel",
		114: "lint_applet", 116: "ki_insertion", 117: "retrieve_content", 118: "critique",
		119: "findings", 120: "browser_mouse_up", 121: "browser_mouse_down", 122: "workspace_api",
		123: "browser_list_network_requests", 124: "browser_get_network_request", 125: "browser_refresh_page", 126: "edit_notebook",
		127: "invoke_subagent", 128: "write_blob", 129: "read_notebook", 130: "propose_ai_comments",
		131: "start_code_review", 132: "generic", 133: "set_up_cloud_sql", 134: "execute_notebook",
		135: "cloud_sql_update_schema", 136: "rpc_action", 137: "cloud_sql_execute_sql", 138: "ask_question",
		139: "directory_rules", 140: "tool_search",
	}
}

// stepStatuses are exa.cortex_pb.CortexStepStatus, lower-cased without
// the prefix.
func stepStatuses() map[int64]string {
	return map[int64]string{
		0: "unspecified", 1: "pending", 2: "running", 3: "done",
		4: "invalid", 5: "cleared", 6: "canceled", 7: "error",
		8: "generating", 9: "waiting", 11: "queued", 12: "interrupted",
	}
}
