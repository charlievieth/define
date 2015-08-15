package define

import (
	"errors"
	"go/ast"
	"go/build"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"sync"

	"golang.org/x/tools/go/types"

	_ "golang.org/x/tools/go/gcimporter"
)

type context struct {
	filepath string
	test     bool
	off      int
	src      []byte
	ctx      build.Context
}

func (c *context) objectOf(id *ast.Ident) (types.Object, error) {
	info, err := c.checkTypes(filepath.Dir(c.filepath))
	if err != nil {
		return nil, err
	}
	if obj := info.ObjectOf(id); obj != nil {
		return obj, nil
	}
	return nil, errors.New("cannot find object")
}

func (c *context) checkTypes(dirname string) (*types.Info, error) {
	files, fset, err := c.parseSourceFiles(dirname)
	if files == nil {
		return nil, err
	}
	info := &types.Info{
		Defs: make(map[*ast.Ident]types.Object),
		Uses: make(map[*ast.Ident]types.Object),
	}
	conf := types.Config{}
	_, pkgErr := conf.Check(dirname, fset, files, info)
	return info, pkgErr
}

func (c *context) parseSourceFiles(dirname string) ([]*ast.File, *token.FileSet, error) {
	names, err := c.PkgFiles(dirname)
	if err != nil {
		return nil, nil, err
	}
	files := make([]*ast.File, 0, len(names))
	fset := token.NewFileSet()
	mu := &sync.Mutex{}
	wg := &sync.WaitGroup{}
	wg.Add(len(names))
	var first error
	for _, name := range names {
		path := filepath.Join(dirname, name)
		var src []byte
		if path == c.filepath {
			src = c.src
		}
		go func(path string, src []byte) {
			defer wg.Done()
			af, err := parseFile(fset, path, src)
			mu.Lock()
			switch {
			case af != nil:
				files = append(files, af)
			case err != nil && first == nil:
				first = err
			}
			mu.Unlock()
		}(path, src)
	}
	if files == nil && first == nil {
		first = errors.New("error parsing directory: " + dirname)
	}
	return files, fset, first
}

func parseFile(fset *token.FileSet, path string, src []byte) (*ast.File, error) {
	af, err := parser.ParseFile(fset, path, src, parser.ParseComments)
	if af == nil {
		return nil, err
	}
	return af, nil
}

func (c *context) PkgFiles(dir string) ([]string, error) {
	f, err := os.Open(dir)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	names, err := f.Readdirnames(-1)
	if err != nil {
		return nil, err
	}
	list := make([]string, 0, len(names))
	for _, name := range list {
		if c.MatchFile(dir, name) {
			list = append(list, name)
		}
	}
	return list, nil
}

func (c *context) MatchFile(dir, name string) (match bool) {
	if isGoSource(name, c.test) {
		match, _ = c.ctx.MatchFile(dir, name)
	}
	return match
}

func isGoSource(s string, includeTest bool) bool {
	return len(s) > len(".go") && s[0] != '_' && s[0] != '.' &&
		hasSuffix(s, ".go") && (includeTest || !hasSuffix(s, "_test.go"))
}

func hasPrefix(s, prefix string) bool {
	return len(s) >= len(prefix) && s[0:len(prefix)] == prefix
}

func hasSuffix(s, suffix string) bool {
	return len(s) >= len(suffix) && s[len(s)-len(suffix):] == suffix
}
