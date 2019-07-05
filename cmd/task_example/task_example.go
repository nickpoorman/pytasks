package main

import (
	"github.com/nickpoorman/go-python3"
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
