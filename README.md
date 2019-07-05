# pytasks

[![GoDoc](https://godoc.org/github.com/nickpoorman/pytasks?status.svg)](https://godoc.org/github.com/nickpoorman/pytasks)
[![CircleCI](https://circleci.com/gh/nickpoorman/pytasks.svg?style=svg)](https://circleci.com/gh/nickpoorman/pytasks)

Python task execution in Go.

This project embeds Python via CGO and aims to make the Python GIL handling more straight forward.

<!-- ----------------------------------------------------------------------------------------------- -->

## Installation

Add the package to your `go.mod` file:

    require github.com/nickpoorman/pytasks master

Or, clone the repository:

    git clone --branch master https://github.com/nickpoorman/pytasks.git $GOPATH/src/github.com/nickpoorman/pytasks

A complete example:

```bash
mkdir pytasks-app && cd pytasks-app

cat > go.mod <<-END
  module my-dataframe-app

  require (
      github.com/nickpoorman/pytasks master
      github.com/DataDog/go-python3 master
  )
END

cat > main.go <<-END
    package main

    import (
        "github.com/DataDog/go-python3"
        "github.com/nickpoorman/pytasks"
    )

    func main() {
        py := pytasks.GetPythonSingleton()
        fooModule, err := py.ImportModule("foo")
        if err != nil {
            panic(err)
        }

        // Start task 1 - this will spawn a new goroutine
        wg1, err := py.NewTask(func() {
            odds := fooModule.GetAttrString("print_odds")
            defer odds.DecRef()

            odds.Call(python3.PyTuple_New(0), python3.PyDict_New())
        })
        if err != nil {
            panic(err)
        }

        // Start task 2 - this will spawn a new goroutine
        wg2, err := py.NewTask(func() {
            even := fooModule.GetAttrString("print_even")
            defer even.DecRef()

            even.Call(python3.PyTuple_New(0), python3.PyDict_New())
        })
        if err != nil {
            panic(err)
        }

        wg1.Wait()
        wg2.Wait()

        // At this point we know we won't need Python anymore in this
        // program, we can restore the state and lock the GIL to perform
        // the final operations before exiting.
        py.Finalize()
    }
END

go run main.go
```

<!-- ----------------------------------------------------------------------------------------------- -->

## Usage

See the [task_example.go](cmd/task_example/task_example.go) or [tests and benchamrks](python_test.go) for usage examples.

## License

(c) 2019 Nick Poorman. Licensed under the Apache License, Version 2.0.
