package parser

// AgentHooks are what a parser declares when its secondary source records
// events in another form or at another moment than its primary store: the
// deduplicator's per-agent hooks. It stands for that package's AgentHooks
// type, which this declaration becomes an alias of.
type AgentHooks struct{}

// HooksDeclarer is implemented by a parser whose secondary rows the
// deduplicator matches under the parser's own hooks. The registry gathers
// them by agent, so no two parsers import each other to supply them.
type HooksDeclarer interface {
	// DedupHooks returns the parser's hooks.
	DedupHooks() AgentHooks
}
