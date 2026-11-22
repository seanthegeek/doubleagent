package glob

// curatedPatterns are the constructs the tables use, in the forms POSIX
// defines and the common POSIX shells agree on.
func curatedPatterns() []string {
	return []string{
		"*", "?", "a*", "*a", "a?c", "a*b*c", "[ab]", "[!ab]", "[a-c]", "[!a-c]", "[]a]",
		"[!]a]", "[a-]", "[]-a]", "[z-a]", "[[:alpha:]]", "[![:digit:]]", "[]", "[!]",
		"*/x", "*.json", ".h*/auth.json", "[!.]*", ".[!e]*", "[0-9a-f][0-9a-f]", "a/*",
		"*/[Zz]ed/node", "[.]*", "",
	}
}

// curatedSubjects are the paths the curated patterns are tried against.
func curatedSubjects() []string {
	return []string{
		"", "a", "b", "c", "z", "-", "]", "[", "^", `\`, `\^`, "*", "?", ".", "x", `x\`,
		"ab", "abc", "aXbYc", "a/x", "a/b/x", ".hidden", ".h/auth.json", ".env", ".e",
		"0f", "x/Zed/node", "y/zed/node", "a/zed/node/z", "a.json", "a/b.json", "[a]",
		"x[", "[a", "!", "1",
	}
}
