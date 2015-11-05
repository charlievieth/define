# define
Find where Go symbols are defined.  Named and anonymous imports are supported.  Currently, support for interfaces and builtin types is missing.  Also the code is bit of a mess at the moment, and still relies on the now deprecated [types](https://godoc.org/golang.org/x/tools/go/types) package.

This was built to support my project [mgo](https://github.com/charlievieth/mgo), which aims to be replace GoSublime's backend and eventually GoSublime itself.
