# eloquent

> Generate fluent APIs in Go.

A "fluent" API is sometimes called the "builder pattern": method chaining is used to optionally set values on an object.

Such methods are tedius to write by-hand, because there is no logic.

This tool writes the methods for you. Invoke with `go generate`, or manually.

## Usage

```go
// foo.go
//go:generate eloquent $GOFILE

type Foo struct {
    // First doc comment.
    First string
    // Second doc comment.
    Second bool
    Third int
    ignored float32
}
```

`go generate`

```go
// foo_eloquent.go

// WithFirst doc comment.
func (f Foo) WithFirst(s string) Foo {
    f.First = s
    return f
}

// WithSecond doc comment.
func (f Foo) WithSecond(b string) Foo {
    f.Second = b
    return f
}

func (f Foo) WithThird(n string) Foo {
    f.Third = n
    return f
}
```

```go
// main.go

func main() {
    foo := NewFoo().
        WithFirst("first").
        WithSecond(false).
        WithThird(3)
}
```
