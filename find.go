package define

import (
	"bytes"
	"errors"
	"fmt"
	"go/ast"
	"go/token"
)

var (
	_ = fmt.Sprint("...")
)

type posFinder interface {
	Candidate(b []byte) bool
	Find(af *ast.File, fset *token.FileSet) *token.Position
}

// TODO: Remove if not used
type posVistor interface {
	Pos() token.Pos
	Visit(node ast.Node) (w ast.Visitor)
}

// Finds top-level (global) declarations
type declFinder struct {
	Name string
}

func (f declFinder) Candidate(b []byte) bool {
	return bytes.Contains(b, []byte(f.Name))
}

func (f declFinder) Find(af *ast.File, fset *token.FileSet) *token.Position {
	if af == nil || fset == nil {
		return nil
	}
	if s := af.Scope; s != nil {
		if o := s.Lookup(f.Name); o != nil {
			return positionFor(o.Pos(), fset)
		}
	}
	return nil
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

type methodFinder struct {
	Name     string
	TypeName string
}

func (f methodFinder) Candidate(b []byte) bool {
	if n := bytes.Index(b, []byte(f.TypeName)); n != -1 {
		return bytes.Index(b[n:], []byte(f.Name)) != -1
	}
	return false
}

func (f methodFinder) Find(af *ast.File, fset *token.FileSet) *token.Position {
	if af == nil || fset == nil {
		return nil
	}
	v := methodVisitor{
		Name:     f.Name,
		TypeName: f.TypeName,
	}
	ast.Walk(&v, af)
	return positionFor(v.pos, fset)
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

// TODO: deprecate in favor of identFinder
type varFinder struct {
	Name string
}

func (f varFinder) Candidate(b []byte) bool {
	return bytes.Contains(b, []byte(f.Name))
}

func (f varFinder) Find(af *ast.File, fset *token.FileSet) *token.Position {
	if af == nil || fset == nil {
		return nil
	}
	if s := af.Scope; s != nil {
		if o := s.Lookup(f.Name); o != nil {
			return positionFor(o.Pos(), fset)
		}
	}
	// TODO: check with varVisitor ???
	return nil
}

// TOOD: Remove if unused
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

type fieldFinder struct {
	Name   string
	Parent string
}

func (f fieldFinder) Candidate(b []byte) bool {
	if n := bytes.Index(b, []byte(f.Parent)); n != -1 {
		return bytes.Index(b[n:], []byte(f.Name)) != -1
	}
	return false
}

func (f fieldFinder) Find(af *ast.File, fset *token.FileSet) *token.Position {
	if af == nil || fset == nil {
		return nil
	}
	// WARN: Debug only!
	if f.Parent == "" {
		panic("fieldFinder: parent is required")
	}
	v := fieldVisitor{
		Name:   f.Name,
		Parent: f.Parent,
	}
	ast.Walk(&v, af)
	return positionFor(v.pos, fset)
}

// Finds struct fields
type fieldVisitor struct {
	Name   string
	Parent string
	pos    token.Pos
	field  bool
}

func (v *fieldVisitor) Pos() token.Pos {
	return v.pos
}

func (v *fieldVisitor) Visit(node ast.Node) ast.Visitor {
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

// Finds package doc file.
type docFinder struct {
	// TODO: This can be done faster as a seperate find/parse routine - we don't
	// need the whole file.
}

// Returns if b contains a comment before the word 'package'.
func (f docFinder) Candidate(b []byte) bool {
	if n := bytes.Index(b, []byte("package")); n > 2 {
		if n = bytes.LastIndex(b[:n], []byte{'\n'}); n > 1 {
			return bytes.Contains(b[:n], []byte("//")) ||
				bytes.Contains(b[:n], []byte("/*"))
		}
	}
	return false
}

func (d docFinder) Find(af *ast.File, fset *token.FileSet) *token.Position {
	if af.Comments != nil && len(af.Comments) != 0 {
		p := fset.Position(af.Comments[0].Pos())
		return &p
	}
	return nil
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

// positionFor, returns the Position for Pos p in FileSet fset.
func positionFor(p token.Pos, fset *token.FileSet) *token.Position {
	if p != token.NoPos && fset != nil {
		if f := fset.File(p); f != nil {
			// Prevent panic
			if f.Base() <= int(p) && int(p) <= f.Base()+f.Size() {
				p := f.Position(p)
				return &p
			}
		}
	}
	return nil
}
