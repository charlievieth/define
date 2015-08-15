package define

import (
	"errors"
	"fmt"
	"go/ast"
	"go/token"
)

var (
	_ = fmt.Sprint("...")
)

type PosVistor interface {
	Pos() token.Pos
	Visit(node ast.Node) (w ast.Visitor)
}

type constVistor struct {
	Name string
	pos  token.Pos
}

func (v *constVistor) Pos() token.Pos {
	return v.pos
}

// Identical to varVisitor.Visit()
func (v *constVistor) Visit(node ast.Node) ast.Visitor {
	if node == nil || v.pos != token.NoPos {
		return nil
	}
	// TODO: check if the ValueSpec is a constant?
	n, ok := node.(*ast.ValueSpec)
	if !ok || n.Names == nil {
		return v
	}
	for _, id := range n.Names {
		if id != nil && id.Name == v.Name {
			v.pos = id.Pos()
			return nil
		}
	}
	return v
}

type funcVistor struct {
	Name string
	pos  token.Pos
}

func (v *funcVistor) Pos() token.Pos {
	return v.pos
}

func (v *funcVistor) Visit(node ast.Node) ast.Visitor {
	if node == nil || v.pos != token.NoPos {
		return nil
	}
	n, ok := node.(*ast.FuncDecl)
	if ok && n.Name != nil && n.Name.Name == v.Name && n.Recv == nil {
		v.pos = n.Pos()
		return nil
	}
	return v
}

type methodVisitor struct {
	Name     string
	TypeName string
	recv     *recvVisitor
	pos      token.Pos
}

func (v *methodVisitor) Pos() token.Pos {
	return v.pos
}

func (v *methodVisitor) Visit(node ast.Node) ast.Visitor {
	if node == nil || v.pos != token.NoPos {
		return nil
	}
	n, ok := node.(*ast.FuncDecl)
	if ok && n.Name != nil && n.Name.Name == v.Name && n.Recv != nil {
		if v.methodOf(n.Recv) {
			v.pos = n.Pos()
			return nil
		}
	}
	return v
}

func (v *methodVisitor) methodOf(list *ast.FieldList) bool {
	if v.recv == nil {
		v.recv = &recvVisitor{Name: v.TypeName}
	} else {
		v.recv.depth = 0
		v.recv.found = false
	}
	ast.Walk(v.recv, list)
	return v.recv.found
}

// Find the name of a method reciever
type recvVisitor struct {
	Name  string // Type name
	found bool
	depth int
}

func methodOf(name string, list *ast.FieldList) bool {
	if list == nil {
		return false
	}
	rv := &recvVisitor{Name: name}
	ast.Walk(rv, list)
	return rv.found
}

func (v *recvVisitor) Visit(node ast.Node) ast.Visitor {
	// TODO: MaxDepth can probably be reduced to 5
	const MaxDepth = 10
	if v.found || node == nil || v.depth > MaxDepth {
		return nil
	}
	id, ok := node.(*ast.Ident)
	if ok && id.Name == v.Name {
		v.found = true
		return nil
	}
	v.depth++
	return v
}

type typeVistor struct {
	Name string
	pos  token.Pos
}

func (t *typeVistor) Pos() token.Pos {
	return t.pos
}

func (v *typeVistor) Visit(node ast.Node) ast.Visitor {
	if node == nil || v.pos != token.NoPos {
		return nil
	}
	if id, ok := node.(*ast.Ident); ok {
		if id.Name == v.Name && id.Obj != nil {
			if _, ok := id.Obj.Decl.(*ast.TypeSpec); ok {
				v.pos = id.Pos()
				return nil
			}
		}
	}
	return v
}

type varVistor struct {
	Name string
	pos  token.Pos
}

func (v *varVistor) Pos() token.Pos {
	return v.pos
}

func (v *varVistor) Visit(node ast.Node) ast.Visitor {
	if node == nil || v.pos != token.NoPos {
		return nil
	}
	n, ok := node.(*ast.ValueSpec)
	if ok && n.Names != nil {
		for _, id := range n.Names {
			if id != nil && id.Name == v.Name {
				v.pos = id.Pos()
				return nil
			}
		}
	}
	return v
}

type structVisitor struct {
	Name   string
	Parent string
	pos    token.Pos
	field  bool
}

func (v *structVisitor) Pos() token.Pos {
	return v.pos
}

func (v *structVisitor) Visit(node ast.Node) ast.Visitor {
	if node == nil || v.pos != token.NoPos {
		return nil
	}
	if id, ok := node.(*ast.Ident); ok && id.Obj != nil {
		switch id.Obj.Decl.(type) {
		case *ast.TypeSpec:
			v.field = (id.Name == v.Parent)
		case *ast.Field:
			if v.field && id.Name == v.Name {
				if _, ok := id.Obj.Decl.(*ast.Field); ok {
					v.pos = id.Pos()
					return nil
				}
			}
		}
	}
	return v
}

// TODO: Remove.  Just find the file with top-level comments, if any.
type packageVistor struct {
	Name string
	pos  token.Pos
}

func (v *packageVistor) Pos() token.Pos {
	return v.pos
}

func (v *packageVistor) Visit(node ast.Node) ast.Visitor {
	return nil
}

type posVisiter struct {
	id    *ast.Ident
	pos   token.Pos
	found bool
}

func (v *posVisiter) Visit(node ast.Node) (w ast.Visitor) {
	if node == nil || v.found {
		return nil
	}
	if node.Pos() <= v.pos && v.pos <= node.End() {
		if vv, ok := node.(*ast.Ident); ok {
			v.id = vv
			v.found = true
		}
	}
	return v
}

func identAtPos(af *ast.File, pos token.Pos) (*ast.Ident, error) {
	if af == nil {
		return nil, errors.New("nil ast.File")
	}
	v := posVisiter{
		pos: pos,
	}
	ast.Walk(&v, af)
	if v.id == nil {
		return nil, errors.New("unable to find ident")
	}
	return v.id, nil
}
