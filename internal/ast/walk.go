package ast

// Walk calls fn for n and then, if fn returns true, for each of its children in
// source order. A single traversal here keeps every pass — resolution, checks,
// tooling — from reimplementing the child lists and quietly missing one.
func Walk(n Node, fn func(Node) bool) {
	if n == nil || !fn(n) {
		return
	}
	for _, child := range Children(n) {
		Walk(child, fn)
	}
}

// Children returns n's child nodes in source order, skipping absent optional
// ones so callers never receive a nil Node.
//
// Optional children are declared with concrete pointer types (*Identifier,
// *Block, ...), and a nil one of those becomes a NON-nil Node interface holding
// a nil pointer. Appending it would hand callers something that passes a nil
// check and then panics on the first method call. The typed helpers below are
// what keep that from happening: each checks its own concrete pointer before
// widening it to Node, so the compiler picks the right check at every call.
func Children(n Node) []Node {
	var out []Node

	node := func(c Node) {
		if c != nil {
			out = append(out, c)
		}
	}
	nodes := func(cs []Node) {
		for _, c := range cs {
			node(c)
		}
	}
	ident := func(c *Identifier) {
		if c != nil {
			out = append(out, c)
		}
	}
	block := func(c *Block) {
		if c != nil {
			out = append(out, c)
		}
	}
	list := func(c *ExpressionList) {
		if c != nil {
			out = append(out, c)
		}
	}

	switch n := n.(type) {
	case *Program:
		nodes(n.Nodes)
	case *Block:
		nodes(n.Nodes)
	case *ExpressionList:
		nodes(n.Elements)

	case *Array:
		list(n.List)
	case *Dictionary:
		for _, p := range n.Pairs {
			node(p.Key)
			node(p.Value)
		}

	case *Let:
		ident(n.Name)
		node(n.Value)
	case *Var:
		ident(n.Name)
		node(n.Value)
	case *Assign:
		node(n.Name)
		node(n.Right)

	case *Prefix:
		node(n.Right)
	case *Infix:
		node(n.Left)
		node(n.Right)
	case *Subscript:
		node(n.Left)
		node(n.Index)
	case *Pipe:
		node(n.Left)
		node(n.Right)
	case *Is:
		node(n.Left)
		ident(n.Right)
	case *As:
		node(n.Left)
		ident(n.Right)

	case *If:
		node(n.Condition)
		block(n.Then)
		block(n.Else)
	case *Switch:
		node(n.Control)
		for _, c := range n.Cases {
			if c != nil {
				out = append(out, c)
			}
		}
		block(n.Default)
	case *SwitchCase:
		list(n.Values)
		node(n.Guard)
		block(n.Body)
	case *TypeCase:
		ident(n.Name)
	case *For:
		if n.Arguments != nil {
			for _, e := range n.Arguments.Elements {
				ident(e)
			}
		}
		node(n.Enumerable)
		block(n.Body)
	case *While:
		node(n.Condition)
		block(n.Body)
	case *Return:
		node(n.Value)

	case *Function:
		for _, p := range n.Parameters {
			if p != nil {
				out = append(out, p)
			}
		}
		ident(n.ReturnType)
		block(n.Body)
	case *FunctionParameter:
		ident(n.Name)
		ident(n.Type)
		node(n.Default)
	case *FunctionCall:
		node(n.Function)
		list(n.Arguments)

	case *Module:
		ident(n.Name)
		block(n.Body)
	case *Access:
		node(n.Left)
		ident(n.Name)

	case *Interpolation:
		for _, part := range n.Parts {
			node(part)
		}

		// Leaves: Identifier, Integer, Float, String, Atom, Boolean, Nil,
		// Placeholder, Break, Continue, Import, Bad, IdentifierList.
	}

	return out
}
