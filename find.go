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
	id1   *ast.Ident
	id2   *ast.Ident
	pos1  token.Pos
	pos2  token.Pos
	found bool
}

func (v *posVisiter) Visit(node ast.Node) (w ast.Visitor) {
	if node == nil {
		return v
	}
	if v.found {
		return nil
	}
	pos := node.Pos()
	end := node.End()
	if pos <= v.pos1 && v.pos1 <= end {
		if vv, ok := node.(*ast.Ident); ok {
			v.id1 = vv
			if v.pos2 == 0 || v.id2 != nil {
				v.found = true
			}
		}
	}
	if v.pos2 != 0 && pos <= v.pos2 && v.pos2 <= end {
		if vv, ok := node.(*ast.Ident); ok {
			v.id2 = vv
			if v.id1 != nil {
				v.found = true
			}
		}
	}
	return v
}

func identAtPos(af *ast.File, curr, prev token.Pos) (*ast.Ident, *ast.Ident, error) {
	if af == nil {
		return nil, nil, errors.New("nil ast.File")
	}
	v := posVisiter{
		pos1: curr,
		pos2: prev,
	}
	ast.Walk(&v, af)
	if v.id1 == nil && v.id2 == nil {
		return nil, nil, errors.New("unable to find ident")
	}
	return v.id1, v.id2, nil
}

type offsetVisitor struct {
	pos  token.Pos
	node ast.Node
}

func (v offsetVisitor) found(start, end token.Pos) bool {
	return start <= v.pos && v.pos <= end
}

func (v *offsetVisitor) Visit(node ast.Node) (w ast.Visitor) {
	if node == nil || v.node != nil {
		return nil
	}
	if node.End() < v.pos {
		return v
	}
	var start token.Pos
	switch n := node.(type) {
	case *ast.Ident:
		start = n.NamePos
	case *ast.SelectorExpr:
		start = n.Sel.NamePos
	case *ast.ImportSpec:
		start = n.Pos()
	case *ast.StructType:
		// TODO: Remove if unnecessary
		if n.Fields == nil {
			break
		}
		// Look for anonymous bare field.
		for _, field := range n.Fields.List {
			if field.Names != nil {
				continue
			}
			t := field.Type
			if pt, ok := field.Type.(*ast.StarExpr); ok {
				t = pt.X
			}
			if id, ok := t.(*ast.Ident); ok {
				if v.found(id.NamePos, id.End()) {
					v.node = id
					return nil
				}
			}
		}
		return v
	default:
		return v
	}
	if v.found(start, node.End()) {
		v.node = node
		return nil
	}
	return v
}

// nodeAtOffset, returns the ast.Node for the given offset.
// Node types: *ast.Ident, *ast.SelectorExpr, *ast.ImportSpec
func nodeAtOffset(af *ast.File, fset *token.FileSet, offset int) (ast.Node, error) {
	file := fset.File(af.Pos())
	if file == nil {
		return nil, errors.New("ast.File not in token.FileSet")
	}
	// Prevent file.Pos from panicking.
	if offset < 0 || file.Size() < offset {
		return nil, fmt.Errorf("invalid offset: %d", offset)
	}
	v := &offsetVisitor{pos: file.Pos(offset)}
	ast.Walk(v, af)
	if v.node == nil {
		return nil, fmt.Errorf("no node at offset: %d", offset)
	}
	return v.node, nil
}
