package define

import (
	"bytes"
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
	dirname  string
	incTest  bool // include test files
	src      []byte
	ctx      *build.Context

	info    *types.Info
	infoErr error

	af    *ast.File   // Source file
	files []*ast.File // Package files
	fset  *token.FileSet
}

func newContext(filename string, af *ast.File, fset *token.FileSet, ctx *build.Context) *context {
	if ctx == nil {
		ctx = &build.Default
	}
	name := filepath.Clean(filename)
	c := context{
		filename: name,
		dirname:  filepath.Dir(name),
		incTest:  hasSuffix(name, "_test.go"),
		ctx:      ctx,
		af:       af,
		fset:     fset,
		files:    []*ast.File{af},
	}
	c.parseTargetDir()
	return &c
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
	if err := c.parseSourcePkg(dirname); err != nil {
		return err
	}
	info := &types.Info{
		Defs: make(map[*ast.Ident]types.Object),
		Uses: make(map[*ast.Ident]types.Object),
	}
	conf := types.Config{}
	if _, err := conf.Check(dirname, c.fset, c.files, info); err != nil {
		// Return error only if missing type info.
		if len(info.Defs) == 0 || len(info.Uses) == 0 {
			c.infoErr = err
			return c.infoErr
		}
	}
	c.info = info
	return nil
}

func (c *context) findObjectPos(o *Object) (*token.Position, error) {
	paths, err := c.pkgPaths(o.PkgPath)
	if err != nil {
		return nil, err
	}
	for _, path := range paths {
		names, err := c.pkgFiles(path)
		if err != nil {
			continue
		}
		_ = names
	}

	return nil, nil
}

// WARN: holy shit rename this thing
func (c *context) findObjectInPkg(path string, o *Object) (*token.Position, error) {
	// Not really names - actually full file paths
	names, err := c.pkgFiles(path)
	if err != nil {
		return nil, err
	}
	for _, name := range names {
		b, err := ioutil.ReadFile(name)
		if err != nil {
			continue
		}
		if !bytes.Contains(b, []byte(o.Name)) {
			continue
		}

	}
	return nil, nil
}

func (c *context) pkgPath(name string) (string, error) {
	for _, dir := range c.ctx.SrcDirs() {
		path := filepath.Join(dir, name)
		if fi, err := os.Stat(name); err != nil && fi.IsDir() {
			return path, nil
		}
	}
	return "", errors.New("package path not found")
}

func (c *context) pkgPaths(name string) (paths []string, err error) {
	for _, dir := range c.ctx.SrcDirs() {
		path := filepath.Join(dir, name)
		if fi, err := os.Stat(name); err != nil && fi.IsDir() {
			paths = append(paths, path)
		}
	}
	if len(paths) == 0 {
		err = errors.New("package path not found")
	}
	return
}

func (c *context) parseTargetDir() error {
	srcs, err := c.readDirSource(c.filename)
	if err != nil {
		return err
	}
	if len(srcs) != 0 {
		files, _ := parseDir(srcs, c.fset)
		c.files = append(c.files, files...)
	}
	return nil
}

func parseDir(srcs map[string][]byte, fset *token.FileSet) ([]*ast.File, error) {
	var first error
	files := make([]*ast.File, 0, len(srcs))
	mu := new(sync.Mutex)
	wg := new(sync.WaitGroup)
	wg.Add(len(srcs))
	for name, src := range srcs {
		go func(name string, src []byte) {
			defer wg.Done()
			af, err := parser.ParseFile(fset, name, src, parser.ParseComments)
			mu.Lock()
			if af != nil {
				files = append(files, af)
			}
			if err != nil && first == nil {
				first = err
			}
			mu.Unlock()
		}(name, src)
	}
	wg.Wait()
	return files, first
}

func (c *context) readDirSource(skip string) (map[string][]byte, error) {
	names, err := c.pkgFiles(c.dirname)
	if err != nil {
		return nil, err
	}
	srcs := make(map[string][]byte)
	for _, name := range names {
		if name != skip {
			if src, _ := ioutil.ReadFile(name); src != nil {
				srcs[name] = src
			}
		}
	}
	return srcs, nil
}

func (c *context) pkgFiles(dir string) ([]string, error) {
	f, err := os.Open(dir)
	if err != nil {
		return nil, err
	}
	names, err := f.Readdirnames(-1)
	if err != nil {
		f.Close()
		return nil, err
	}
	f.Close()
	var n int
	for i := 0; i < len(names); i++ {
		if c.MatchFile(dir, names[i]) {
			names[n] = filepath.Join(dir, names[i])
			n++
		}
	}
	return names[:n], nil
}

func (c *context) readPkgSource(dirname string) (map[string][]byte, error) {
	names, err := c.pkgFiles(dirname)
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

func parseFile(filename string, src []byte) (*ast.File, *token.FileSet, error) {
	fset := token.NewFileSet()
	af, err := parser.ParseFile(fset, filename, src, parser.ParseComments)
	if af == nil {
		return nil, nil, err
	}
	return af, fset, nil
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

func (c *context) parseSourcePkg(dirname string) error {
	srcs, err := c.readPkgSource(dirname)
	if err != nil {
		return err
	}

	c.fset = token.NewFileSet()
	var first error

	mu := new(sync.Mutex)
	wg := new(sync.WaitGroup)
	wg.Add(len(srcs))

	for name, src := range srcs {
		go func(name string, src []byte) {
			defer wg.Done()
			af, err := parser.ParseFile(c.fset, name, src, parser.ParseComments)
			mu.Lock()
			switch {
			case af != nil:
				c.files = append(c.files, af)
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

	if c.af == nil {
		return first
	}
	return nil
}
