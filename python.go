package pytasks

import (
	"errors"
	"fmt"
	"runtime"
	"sync"

	"github.com/DataDog/go-python3"
)

var singletonOnce sync.Once
var pythonSingletonInstance *pythonSingleton

type Tuple struct {
	Result interface{}
	Err    error
}

// PythonSingleton is an interface to the pythonSingleton instance
type PythonSingleton interface {
	ImportModule(name string) (*python3.PyObject, error)
	NewTask(task func()) (*sync.WaitGroup, error)
	Finalize(from ...string) error
}

type pythonSingleton struct {
	taskWG    sync.WaitGroup
	stopWG    sync.WaitGroup
	stoppedWG sync.WaitGroup

	lock     sync.Mutex
	modules  map[string]*python3.PyObject
	stopped  bool
	stopOnce sync.Once
}

type PythonSingletonOption func(ps *pythonSingleton) error

func WithModules(modules []string) PythonSingletonOption {
	return func(ps *pythonSingleton) error {
		for _, m := range modules {
			_, err := ps.ImportModule(m)
			if err != nil {
				return err
			}
		}
		return nil
	}
}

// GetPythonSingleton returns the existing pythonSingleton
// or creates a new one if one has not been created yet.
func GetPythonSingleton(opts ...PythonSingletonOption) PythonSingleton {
	singletonOnce.Do(func() {
		ps := &pythonSingleton{
			modules: make(map[string]*python3.PyObject),
		}
		tuple := ps.initPython(opts...)
		startedWG := tuple.Result.(*sync.WaitGroup)
		startedWG.Wait()
		pythonSingletonInstance = ps
	})
	return pythonSingletonInstance
}

// This is a special case. The thread that inits python needs to also shut it down.
// This function returns a WaitGroup that should be waited on for startup to finish.
func (ps *pythonSingleton) initPython(opts ...PythonSingletonOption) *Tuple {
	var startedWG sync.WaitGroup
	startedWG.Add(1)
	ps.stoppedWG.Add(1)
	ps.stopWG.Add(1)
	result := &Tuple{
		Result: &startedWG,
	}
	go func() {
		runtime.LockOSThread()

		// The following will also create the GIL explicitly
		// by calling PyEval_InitThreads(), without waiting
		// for the interpreter to do that
		python3.Py_Initialize()
		if !python3.Py_IsInitialized() {
			result.Err = errors.New("Error initializing the python interpreter")
			return
		}

		// https://stackoverflow.com/questions/27844676/assertionerror-3-x-only-when-calling-py-finalize-with-threads
		threadingMod := python3.PyImport_ImportModule("threading")
		threadingMod.DecRef()

		for _, opt := range opts {
			err := opt(ps)
			if err != nil {
				result.Err = err
				return
			}
		}

		// Initialize() has locked the the GIL but at this point we don't need it
		// anymore. We save the current state and release the lock
		// so that goroutines can acquire it
		state := python3.PyEval_SaveThread()

		// Trigger startedWG.
		startedWG.Done()

		// Wait until finalize is triggered for the singleton in this thread.
		ps.stopWG.Wait()

		// At this point we know we won't need Python anymore in this
		// program, we can restore the state and lock the GIL to perform
		// the final operations before exiting.
		python3.PyEval_RestoreThread(state)

		python3.Py_Finalize()
		ps.stoppedWG.Done()
	}()

	return result
}

// NewTask creates a new task that will run in python.
// This returns a WaitGroup that will release when it is ready to continue processing and use it's return value
// meaning the Python GIL has been released.
func (ps *pythonSingleton) NewTask(task func()) (*sync.WaitGroup, error) {
	ps.lock.Lock()
	defer ps.lock.Unlock()
	if ps.stopped {
		return nil, errors.New("Finalize has been called on PythonSingleton. No new operations")
	}

	return ps.newTask(task)
}

// newTask will start a task without a lock. This is used internally by public NewTask and ImportModule.
//
// When a goroutine starts, it’s scheduled for execution on one of the GOMAXPROCS
// threads available—see here for more details on the topic. If a goroutine happens
// to perform a syscall or call C code, the current thread hands over the other
// goroutines waiting to run in the thread queue to another thread so they can have
// better chances to run; the current goroutine is paused, waiting for the syscall or
// the C function to return. When this happens, the thread tries to resume the paused
// goroutine, but if this is not possible, it asks the Go runtime to find another
// thread to complete the goroutine and goes to sleep. The goroutine is finally
// scheduled to another thread and it finishes.
//
// 1. Our goroutine starts, performs a C call, and pauses. The GIL is locked.
// 2. When the C call returns, the current thread tries to resume the goroutine, but it fails.
// 3. The current thread tells the Go runtime to find another thread to resume our goroutine.
// 4. The Go scheduler finds an available thread and the goroutine is resumed.
// 5. The goroutine is almost done and tries to unlock the GIL before returning.
// 6. The thread ID stored in the current state is from the original thread and is different from the ID of the current thread.
// 7. Panic!
func (ps *pythonSingleton) newTask(task func()) (*sync.WaitGroup, error) {
	var wg sync.WaitGroup
	wg.Add(1)
	ps.taskWG.Add(1)
	go func() {
		defer ps.taskWG.Done()
		defer wg.Done()

		runtime.LockOSThread()
		defer runtime.UnlockOSThread()

		_gstate := python3.PyGILState_Ensure()
		defer python3.PyGILState_Release(_gstate)

		// save the response from the task
		task()
	}()

	return &wg, nil
}

// Finalize trigger the thread that started python to stop it.
// It will block until Python has stopped.
func (ps *pythonSingleton) Finalize(from ...string) error {
	fmt.Printf("Called Finalize %v\n", from)
	ps.lock.Lock()
	if ps.stopped {
		return errors.New("Finalize already called on PythonSingleton")
	}
	ps.stopped = true
	ps.lock.Unlock()

	ps.stopOnce.Do(func() {
		go func() {
			ps.stopWG.Done()
		}()
		ps.stoppedWG.Wait()
	})
	return nil
}

// ImportModule will import the python module and add it
// to the registry or return an already imported module.
func (ps *pythonSingleton) ImportModule(name string) (*python3.PyObject, error) {
	fmt.Printf("Importing module: %s | existing: %v\n", name, ps.LoadedModuleNames())
	ps.lock.Lock()
	defer ps.lock.Unlock()
	if ps.stopped {
		return nil, errors.New("Finalize has been called on PythonSingleton. No new operations")
	}

	// Return the module if is has already been initialized.
	if mod, ok := ps.modules[name]; ok {
		return mod, nil
	}

	var module *python3.PyObject
	var err error

	taskWG, taskErr := ps.newTask(func() {
		// module = python3.PyImport_ImportModule(name)
		pyName := python3.PyUnicode_FromString(name)
		module = python3.PyImport_Import(pyName)
		if module == nil {
			python3.PyErr_PrintEx(false)
			err = errors.New("error importing module")
		}
	})
	if taskErr != nil {
		return nil, taskErr
	}
	taskWG.Wait()

	// add the module to the loaded modules
	ps.modules[name] = module

	return module, err
}

func (ps *pythonSingleton) syncTask(task func()) error {
	taskWG, taskErr := ps.newTask(task)
	if taskErr != nil {
		return taskErr
	}
	taskWG.Wait()
	return nil
}

func (ps *pythonSingleton) LoadedModuleNames() []string {
	names := make([]string, 0, len(ps.modules))
	for name := range ps.modules {
		names = append(names, name)
	}
	return names
}
