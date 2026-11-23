package parserfacts

import (
	"go/ast"
	"go/token"
	"go/types"
)

// pattern is one file pattern a parser selects artifacts with: a regular
// expression or a path compared with the artifact's relative path, as
// written when every piece of it is constant.
type pattern struct {
	text     string
	regex    bool
	resolved bool
	position token.Pos
}

// patternWalker collects the file patterns of one parser: the kinds its
// Kinds method builds and the tests its Secondary method makes of the
// artifact's relative path.
type patternWalker struct {
	evaluator *evaluator
	kit       string
	found     []pattern
	seen      map[*types.Func]bool
}

// maxCallDepth bounds how deep the walker follows calls from Kinds or
// Secondary.
const maxCallDepth = 4

// kinds collects the patterns of the kinds a function body builds: a
// PatternKind's regular expression, a RelKind's path, and the path tests
// of a RelFuncKind's function or a Kind literal's Wants. It follows the
// calls the body makes to functions outside the kit, so a helper that
// returns some of the kinds counts.
func (w *patternWalker) kinds(body ast.Node, depth int) {
	ast.Inspect(body, func(node ast.Node) bool {
		switch typed := node.(type) {
		case *ast.CallExpr:
			w.kindCall(typed, depth)
		case *ast.CompositeLit:
			w.kindLiteral(typed, depth)
		case *ast.IfStmt:
			if w.evaluator.neverTrue(typed.Cond) {
				if typed.Else != nil {
					w.kinds(typed.Else, depth)
				}
				return false
			}
		}
		return true
	})
}

// kindCall reads one call made while building the kinds.
func (w *patternWalker) kindCall(call *ast.CallExpr, depth int) {
	called := w.evaluator.index.callee(call)
	if called == nil || called.Pkg() == nil {
		return
	}
	if called.Pkg().Path() == w.kit {
		switch called.Name() {
		case "PatternKind":
			w.add(call.Args[1], true, nil)
		case "RelKind":
			w.add(call.Args[1], false, nil)
		case "RelFuncKind":
			w.tests(call.Args[1], nil, depth+1)
		}
		return
	}
	if declaration := w.evaluator.index.bodies[called.Origin()]; declaration != nil && depth < maxCallDepth &&
		!w.seen[called.Origin()] {
		w.seen[called.Origin()] = true
		w.kinds(declaration.Body, depth+1)
	}
}

// kindLiteral reads a parserkit.Kind composite literal's Wants function.
func (w *patternWalker) kindLiteral(literal *ast.CompositeLit, depth int) {
	named, ok := types.Unalias(w.evaluator.index.info.Types[literal].Type).(*types.Named)
	if !ok || named.Obj().Pkg() == nil || named.Obj().Pkg().Path() != w.kit || named.Obj().Name() != "Kind" {
		return
	}
	for _, element := range literal.Elts {
		if pair, ok := element.(*ast.KeyValueExpr); ok {
			if name, ok := pair.Key.(*ast.Ident); ok && name.Name == "Wants" {
				w.tests(pair.Value, nil, depth+1)
			}
		}
	}
}

// add records the patterns an expression can be.
func (w *patternWalker) add(expression ast.Expr, regex bool, bound environment) {
	for _, shape := range w.evaluator.strings(expression, bound, 0) {
		found := pattern{text: render(shape), regex: regex, resolved: true, position: expression.Pos()}
		for _, part := range shape {
			if part.kind != constantPiece {
				found.resolved = false
			}
		}
		if len(shape) > 0 && shape[0].kind == constantPiece {
			found.position = shape[0].position
		}
		w.found = append(w.found, found)
	}
}

// tests collects the path tests of a function given as an expression: a
// function literal, or a declared function, whose first string parameter
// is the relative path.
func (w *patternWalker) tests(function ast.Expr, bound environment, depth int) {
	var body *ast.BlockStmt
	var signature *types.Signature
	switch typed := ast.Unparen(function).(type) {
	case *ast.FuncLit:
		body = typed.Body
		signature, _ = w.evaluator.index.info.Types[typed].Type.(*types.Signature)
	default:
		called := w.functionValue(typed)
		if called == nil {
			return
		}
		if declaration := w.evaluator.index.bodies[called.Origin()]; declaration != nil {
			body, signature = declaration.Body, called.Signature()
		}
	}
	if body == nil || signature == nil {
		return
	}
	inner := environment{}
	for key, value := range bound {
		inner[key] = value
	}
	if signature.Params().Len() > 0 && isString(signature.Params().At(0).Type()) {
		inner[signature.Params().At(0)] = []template{{{kind: relPiece}}}
	}
	w.body(body, inner, depth, true)
}

// functionValue returns the declared function an expression names.
func (w *patternWalker) functionValue(expression ast.Expr) *types.Func {
	switch typed := expression.(type) {
	case *ast.Ident:
		called, _ := w.evaluator.index.info.Uses[typed].(*types.Func)
		return called
	case *ast.SelectorExpr:
		if selection := w.evaluator.index.info.Selections[typed]; selection != nil {
			called, _ := selection.Obj().(*types.Func)
			return called
		}
		called, _ := w.evaluator.index.info.Uses[typed.Sel].(*types.Func)
		return called
	}
	return nil
}

// isRel reports whether an expression is the artifact's relative path.
func (w *patternWalker) isRel(expression ast.Expr, bound environment) bool {
	for _, shape := range w.evaluator.strings(expression, bound, 0) {
		if len(shape) == 1 && shape[0].kind == relPiece {
			return true
		}
	}
	return false
}

// body collects the path tests of a function body: a regular expression
// matched against the path (any regular expression the body matches,
// when top says the body is the test function itself), a comparison of the path with a string, a
// switch on the path, a list holding it, a prefix or suffix of it, and the
// tests of the functions the body calls.
func (w *patternWalker) body(body *ast.BlockStmt, bound environment, depth int, top bool) {
	ast.Inspect(body, func(node ast.Node) bool {
		switch typed := node.(type) {
		case *ast.FuncLit:
			return false
		case *ast.BinaryExpr:
			if typed.Op == token.EQL || typed.Op == token.NEQ {
				w.compared(typed.X, typed.Y, bound)
				w.compared(typed.Y, typed.X, bound)
			}
		case *ast.SwitchStmt:
			if typed.Tag != nil && w.isRel(typed.Tag, bound) {
				w.cases(typed.Body, bound)
			}
		case *ast.CallExpr:
			w.testCall(typed, bound, depth, top)
		}
		return true
	})
}

// compared records other as a path when one is the relative path.
func (w *patternWalker) compared(one, other ast.Expr, bound environment) {
	if w.isRel(one, bound) && !w.isRel(other, bound) {
		w.add(other, false, bound)
	}
}

// cases records the values of each case of a switch on the path.
func (w *patternWalker) cases(body *ast.BlockStmt, bound environment) {
	for _, statement := range body.List {
		if clause, ok := statement.(*ast.CaseClause); ok {
			for _, value := range clause.List {
				w.add(value, false, bound)
			}
		}
	}
}

// testCall reads one call of a path test: a regular expression method or
// a string function given the path, or a call of a function of the
// checked packages, whose tests count with its parameters bound.
func (w *patternWalker) testCall(call *ast.CallExpr, bound environment, depth int, top bool) {
	called := w.evaluator.index.callee(call)
	if called == nil || called.Pkg() == nil {
		return
	}
	given := false
	for _, each := range call.Args {
		given = given || w.isRel(each, bound)
	}
	switch called.Pkg().Path() {
	case "regexp":
		if selector, ok := ast.Unparen(call.Fun).(*ast.SelectorExpr); ok && (given || top) {
			w.add(selector.X, true, bound)
		}
		return
	case "strings":
		if (called.Name() == "HasPrefix" || called.Name() == "HasSuffix") && w.isRel(call.Args[0], bound) {
			w.add(call.Args[1], false, bound)
		}
		return
	case "slices":
		if called.Name() == "Contains" && len(call.Args) == 2 && w.isRel(call.Args[1], bound) {
			for _, shape := range w.evaluator.elements(call.Args[0], bound, 0) {
				w.addTemplate(shape, call.Args[0].Pos())
			}
		}
		return
	}
	declaration := w.evaluator.index.bodies[called.Origin()]
	if declaration == nil || depth >= maxCallDepth {
		return
	}
	w.body(declaration.Body, w.evaluator.bind(called.Signature(), call, bound, 0), depth+1, false)
}

// addTemplate records one plain path template.
func (w *patternWalker) addTemplate(shape template, position token.Pos) {
	found := pattern{text: render(shape), resolved: true, position: position}
	for _, part := range shape {
		if part.kind != constantPiece {
			found.resolved = false
		}
	}
	if len(shape) > 0 && shape[0].kind == constantPiece {
		found.position = shape[0].position
	}
	w.found = append(w.found, found)
}
