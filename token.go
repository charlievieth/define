package define

import (
	"go/scanner"
	"go/token"
)

type tokenPos struct {
	off int         // source code position
	end int         // source code position
	pos token.Pos   // toke fileset pos
	tok token.Token // underlying token
}

func (t tokenPos) Pos() int { return t.off }

func (t tokenPos) End() int { return t.end }

func (t tokenPos) Valid() bool {
	return t.off != 0 && t.end != 0 && t.tok != token.ILLEGAL
}

type iterator struct {
	tokens []tokenPos
	index  int
}

func (i *iterator) Token() tokenPos {
	return i.tokens[i.index]
}

func (i *iterator) Previous() (t tokenPos) {
	if i.index > 0 {
		i.index--
		t = i.tokens[i.index]
	}
	return t
}

func (i *iterator) Next() (t tokenPos) {
	if i.index < len(i.tokens)-1 {
		i.index++
		t = i.tokens[i.index]
	}
	return t
}

func newIterator(file *token.File, fset *token.FileSet, src []byte, offset int) iterator {
	scan := &scanner.Scanner{}
	scan.Init(file, src, nil, 0)
	tokens := make([]tokenPos, 0, 1024)
	index := 0
	for {
		pos, tok, lit := scan.Scan()
		if tok == token.EOF {
			break
		}
		off := fset.Position(pos).Offset
		tokens = append(tokens, tokenPos{
			off: off,
			end: off + len(lit),
			pos: pos,
			tok: tok,
		})
		if offset <= off {
			break
		}
		index++
	}
	return iterator{
		tokens: tokens,
		index:  index,
	}
}

func cursorContext(file *token.File, fset *token.FileSet, src []byte, offset int) (current, parent tokenPos) {
	iter := newIterator(file, fset, src, offset)
	switch iter.Previous().tok {
	case token.PERIOD, token.COMMA, token.LBRACE:
		current = iter.Previous()
	case token.IDENT, token.TYPE, token.CONST, token.VAR, token.FUNC, token.PACKAGE:
		current = iter.Token()
	}
	if iter.Previous().tok == token.PERIOD {
		parent = iter.Previous()
	}
	return current, parent
}
