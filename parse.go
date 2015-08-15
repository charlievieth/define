package define

import (
	"errors"
	"go/ast"
	"go/build"
	"go/parser"
	"go/token"
	"io/ioutil"
	"os"
	"path/filepath"
	"sync"

	"golang.org/x/tools/go/types"

	_ "golang.org/x/tools/go/gcimporter"
)

type context struct {
	filename string
	incTest  bool // include test files
	src      []byte
	ctx      *build.Context

	info    *types.Info
	infoErr error

	af *ast.File
}

func newContext(filename string, src []byte, ctx *build.Context) (*context, error) {
	c := &context{
		filename: filename,
		incTest:  hasSuffix(filename, "_test.go"),
		src:      src,
		ctx:      ctx,
	}
	return c, c.checkTypes(filepath.Dir(filename))
}

func (c *context) objectOf(id *ast.Ident) (types.Object, error) {
	if err := c.checkTypes(filepath.Dir(c.filename)); err != nil {
		return nil, err
	}
	if obj := c.info.ObjectOf(id); obj != nil {
		return obj, nil
	}
	return nil, errors.New("cannot find object")
}

func (c *context) checkTypes(dirname string) error {
	if c.info != nil {
		return c.infoErr
	}
	files, fset, err := c.parseSourceFiles(dirname)
	if files == nil {
		return err
	}
	info := &types.Info{
		Defs: make(map[*ast.Ident]types.Object),
		Uses: make(map[*ast.Ident]types.Object),
	}
	conf := types.Config{}
	if _, err := conf.Check(dirname, fset, files, info); err != nil {
		// Return error only if missing type info.
		if len(info.Defs) == 0 || len(info.Uses) == 0 {
			c.infoErr = err
			return c.infoErr
		}
	}
	c.info = info
	return nil
}

func (c *context) prsSourceFiles(dirname string) ([]*ast.File, *token.FileSet, error) {
	names, err := c.PkgFiles(dirname)
	if err != nil {
		return nil, nil, err
	}

	// Read files here to prevent concurrent disk access.
	var index int
	srcs := make([][]byte, len(names))
	for i := 0; i < len(names); i++ {
		path := filepath.Join(dirname, names[i])
		names[i] = path
		if path == c.filename {
			srcs[i] = c.src
			index = i
		} else {
			if src, _ := ioutil.ReadFile(path); src != nil {
				srcs[i] = src
			}
		}
	}

	// Parse response
	type response struct {
		af  *ast.File
		err error
	}
	reschs := make([]chan response, 0, len(srcs))

	fset := token.NewFileSet()
	for i, name := range names {
		ch := make(chan response)
		reschs = append(reschs, ch)
		go func(name string, src []byte, ch chan response) {
			af, err := parser.ParseFile(fset, name, src, parser.ParseComments)
			ch <- response{af, err}
		}(name, srcs[i], ch)
	}

	var first error
	files := make([]*ast.File, 0, len(srcs))
	for i, ch := range reschs {
		switch r := <-ch; {
		case r.af != nil:
			files = append(files, r.af)
			if i == index {
				c.af = r.af
			}
		case r.err != nil:
			switch {
			case i == index:
				first = r.err
			case first == nil:
				first = r.err
			}
		}
	}
	if c.af == nil {
		return nil, nil, errors.New("error parsing src file: " + first.Error())
	}
	return files, fset, first
}

func (c *context) pSourceFiles(dirname string) ([]*ast.File, *token.FileSet, error) {
	srcs, err := c.readPkgSource(dirname)
	if err != nil {
		return nil, nil, err
	}

	fset := token.NewFileSet()
	var files []*ast.File
	var first error

	mu := new(sync.Mutex)
	wg := new(sync.WaitGroup)
	wg.Add(len(srcs))

	for name, src := range srcs {
		go func(name string, src []byte) {
			defer wg.Done()
			af, err := parser.ParseFile(fset, name, src, parser.ParseComments)
			mu.Lock()
			switch {
			case af != nil:
				files = append(files, af)
				if name == c.filename {
					c.af = af
				}
			case err != nil:
				switch {
				case name == c.filename:
					first = err
				case first == nil:
					first = err
				}
			}
			mu.Unlock()
		}(name, src)
	}
	wg.Wait()

	return files, fset, first
}

// fileSrc == filepath -> filesrc
func parseFiles(fileSrc map[string][]byte) ([]*ast.File, *token.FileSet, error) {
	// Parse response
	type response struct {
		af  *ast.File
		err error
	}
	reschs := make([]chan response, 0, len(fileSrc))

	fset := token.NewFileSet()
	for name, src := range fileSrc {
		ch := make(chan response)
		reschs = append(reschs, ch)
		go func(name string, src []byte, ch chan response) {
			af, err := parser.ParseFile(fset, name, src, parser.ParseComments)
			ch <- response{af, err}
		}(name, src, ch)
	}

	var first error
	files := make([]*ast.File, 0, len(fileSrc))
	for _, ch := range reschs {
		switch r := <-ch; {
		case r.af != nil:
			files = append(files, r.af)
		case r.err != nil && first == nil:
			first = r.err
		}
	}

	return files, fset, first
}

func (c *context) readPkgSource(dirname string) (map[string][]byte, error) {
	names, err := c.PkgFiles(dirname)
	if err != nil {
		return nil, err
	}
	srcs := make(map[string][]byte)
	for _, name := range names {
		path := filepath.Join(dirname, name)
		if path == c.filename {
			srcs[path] = c.src
		} else {
			if src, _ := ioutil.ReadFile(path); src != nil {
				srcs[path] = src
			}
		}
	}
	if len(srcs) == 0 {
		return nil, errors.New("reading source files in directory: " + dirname)
	}
	return srcs, nil
}

func (c *context) parseSourceFiles(dirname string) ([]*ast.File, *token.FileSet, error) {
	names, err := c.PkgFiles(dirname)
	if err != nil {
		return nil, nil, err
	}

	// Read files here to prevent concurrent disk access.
	fileSrc := make(map[string][]byte)
	for _, name := range names {
		path := filepath.Join(dirname, name)
		if path == c.filename {
			fileSrc[path] = c.src
		} else {
			if src, _ := ioutil.ReadFile(path); src != nil {
				fileSrc[path] = src
			}
		}
	}
	if len(fileSrc) == 0 {
		return nil, nil, errors.New("error reading files in: " + dirname)
	}
	return parseFiles(fileSrc)
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
	if len(names) == 0 {
		return nil, errors.New("no files in directory: " + dir)
	}
	list := make([]string, 0, len(names))
	for _, name := range list {
		if c.MatchFile(dir, name) {
			list = append(list, name)
		}
	}
	if len(list) == 0 {
		return nil, errors.New("no Go source files in directory: " + dir)
	}
	return list, nil
}

func (c *context) MatchFile(dir, name string) (match bool) {
	if isGoSource(name, c.incTest) {
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
