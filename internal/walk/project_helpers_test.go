package walk

// projectScope decides RootCheck.OutOfRoot's question for root with probe,
// resolving the root on each call, for tests.
func projectScope(root, project string, probe probes) rootScope {
	return NewRootCheck(root).scope(project, probe)
}

// ProjectOutOfRoot is RootCheck.OutOfRoot for root, resolved on each call.
func ProjectOutOfRoot(root, project string) (string, bool) {
	return NewRootCheck(root).OutOfRoot(project)
}

// ProjectLeavesRoot reports whether RootCheck.Exit refuses project under
// root, resolved on each call.
func ProjectLeavesRoot(root, project string) bool {
	_, leaves := NewRootCheck(root).Exit(project)
	return leaves
}
