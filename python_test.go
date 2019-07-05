package pytasks

import (
	"testing"

	"github.com/DataDog/go-python3"
)

func BechmarkAll(b *testing.B) {
	b.Run("Py", func(b *testing.B) {
		py := GetPythonSingleton()

		fooModule, err := py.ImportModule("foo")
		if err != nil {
			panic(err)
		}
		fooModule2, err := py.ImportModule("foo")
		if err != nil {
			panic(err)
		}
		fooModule2.DecRef()

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			wg1, err := py.NewTask(func() {
				odds := fooModule.GetAttrString("print_odds")
				defer odds.DecRef()

				odds.Call(python3.PyTuple_New(0), python3.PyDict_New())
			})
			if err != nil {
				panic(err)
			}

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
		}
	})

	// At this point we know we won't need Python anymore in this
	// program, we can restore the state and lock the GIL to perform
	// the final operations before exiting.
	err := GetPythonSingleton().Finalize("BenchmarkPy cleanup")
	if err != nil {
		panic(err)
	}
}

func TestAll(t *testing.T) {

	t.Run("TestModulePointer", func(t *testing.T) {
		py := GetPythonSingleton()

		fooModule1, err := py.ImportModule("foo")
		if err != nil {
			panic(err)
		}

		fooModule2, err := py.ImportModule("foo")
		if err != nil {
			panic(err)
		}

		if fooModule1 != fooModule2 {
			t.Fatalf("expected pointers to be the same. got: %p != %p", fooModule1, fooModule2)
		}
	})

	t.Run("TestSingleton", func(t *testing.T) {
		py := GetPythonSingleton()

		py2 := GetPythonSingleton()

		if py != py2 {
			t.Fatalf("not a singleton - expected %p to equal %p", py, py2)
		}
	})

	t.Run("TestSingletonFinalize", func(t *testing.T) {
		py := GetPythonSingleton()

		// At this point we know we won't need Python anymore in this
		// program, we can restore the state and lock the GIL to perform
		// the final operations before exiting.
		err := py.Finalize("TestSingletonFinalize1")
		if err != nil {
			panic(err)
		}

		_, err = py.ImportModule("foo")
		if err == nil {
			t.Fatalf("expected to get an error for ImportModule")
		}

		_, err = py.NewTask(func() {})
		if err == nil {
			t.Fatalf("expected to get an error for NewTask")
		}

		err = py.Finalize("TestSingletonFinalize2")
		if err == nil {
			t.Fatalf("expected to get an error for Finalize")
		}
	})
}
