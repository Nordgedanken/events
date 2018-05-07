package events

import (
	"sync"
	"time"
	"log"
)

type Dispatcher interface {
	On(on string, fn func(interface{}) error)
	Raise(on string, data interface{})
	Wait()
	Close()
}

type EventHappened struct {
	Time time.Time
	On   string
	Data interface{}
}

type EventRepo interface  {
	Set(string, interface{}) error
	GetAll() (map[string][]EventHappened, error)
	Contains(string) bool
}

type listener struct {
	on  string
	fn  func(interface{}) error
}

type ListenerError struct {
	Err error
	On  string
	In  interface{}
	Fn  func(interface{}) error
}

func newError(err error, on string, in interface{}, fn func(interface{}) error) ListenerError {
	return ListenerError{err, on, in, fn}
}

type ErrorListenerFn func(err ListenerError)

func newListener(on string, fn func(interface{}) error) *listener {
	return &listener{
		on: on,
		fn: fn,
	}
}

func (l *listener) execute(e *Events, d interface{}) {
	log.Println("Executing on:", l.on)

	e.wg.Add(1)
	go func(e *Events) {
		err := l.fn(d)
		defer e.wg.Done()

		if err != nil {
			e.errCh <- newError(err, l.on, d, l.fn)
		}
	}(e)
}

type Events struct {
	// Mutex to prevent race conditions within the Emitter.
	*sync.RWMutex
	// Map of listener on event name
	listeners  map[string][]listener
	errorFn   ErrorListenerFn
	wg        sync.WaitGroup
	errCh     chan ListenerError

	eventRepo EventRepo
}

func New() (e *Events) {
	e = new(Events)
	e.RWMutex = new(sync.RWMutex)
	e.listeners = make(map[string][]listener)
	e.errCh = make(chan ListenerError)

	go func(errCh chan ListenerError) {
		for {
			select {
				case err := <- errCh:
					if err.Err != nil {
						e.raiseError(err)
					}
				}
		}
	}(e.errCh)

	return
}

func NewWithErrListener(el ErrorListenerFn) (e *Events){
	e = New()
	e.OnError(el)
	return
}

func (e *Events) OnError(el ErrorListenerFn) {
	e.errorFn = el
	return
}

func (e *Events) AddEventRepo(es EventRepo) {
	e.eventRepo = es
	return
}

func (e *Events) GetEventRepo() EventRepo {
	return e.eventRepo
}


func (e *Events) On(on string, fn func(interface{}) error) {
	l := newListener(on, fn)
	e.RWMutex.Lock()
	e.listeners[on] = append(e.listeners[on], *l)
	e.RWMutex.Unlock()
}

func (e *Events) Remove(on string) {
	e.RWMutex.Lock()
	e.listeners[on] = nil
	e.RWMutex.Unlock()
}

func (e *Events) Raise(on string, data interface{}) {

	if e.eventRepo != nil {
		e.eventRepo.Set(on, data)
	}

	for _, l := range e.listeners[on] {
		l.execute(e, data)
	}
}

func (e *Events) Wait() {
	e.wg.Wait()
}

func (e *Events) Close() {
	close(e.errCh)
}

func (e *Events) raiseError(err ListenerError) {
	if e.errorFn != nil {
		e.wg.Add(1)
		defer e.wg.Done()
		e.errorFn(err)
	}
}
