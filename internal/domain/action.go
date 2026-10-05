package domain

// Action is a typed operation an actor may request. The set is closed: there
// is deliberately no generic command, exec or shell action.
type Action string

// Ordinary capabilities.
const (
	ActionReadFile   Action = "READ_FILE"
	ActionWriteFile  Action = "WRITE_FILE"
	ActionDeleteFile Action = "DELETE_FILE"
	ActionListFiles  Action = "LIST_FILES"
	ActionRunTest    Action = "RUN_TEST"
	ActionGitStatus  Action = "GIT_STATUS"
	ActionGitDiff    Action = "GIT_DIFF"
)

// Consequential actions. They are represented for authorization only; nothing
// in this package performs them.
const (
	ActionCreateWorktree  Action = "CREATE_WORKTREE"
	ActionCreateCommit    Action = "CREATE_COMMIT"
	ActionPush            Action = "PUSH"
	ActionMerge           Action = "MERGE"
	ActionExternalRequest Action = "EXTERNAL_REQUEST"
	ActionAccessSecret    Action = "ACCESS_SECRET"
)

// IsOrdinary reports whether a is one of the ordinary capabilities.
func (a Action) IsOrdinary() bool {
	switch a {
	case ActionReadFile, ActionWriteFile, ActionDeleteFile, ActionListFiles,
		ActionRunTest, ActionGitStatus, ActionGitDiff:
		return true
	default:
		return false
	}
}

// IsConsequential reports whether a is a consequential action.
func (a Action) IsConsequential() bool {
	switch a {
	case ActionCreateWorktree, ActionCreateCommit, ActionPush, ActionMerge,
		ActionExternalRequest, ActionAccessSecret:
		return true
	default:
		return false
	}
}

// IsValid reports whether a is a declared action. Anything else, including
// the zero value, is invalid.
func (a Action) IsValid() bool {
	return a.IsOrdinary() || a.IsConsequential()
}
