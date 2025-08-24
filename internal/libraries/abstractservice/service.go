package abstractservice

import (
	"context"
	"time"
	"sync"
	"os"
	"os/signal"
	"syscall"
	"errors"

	"golang.org/x/sync/errgroup"
)

const defaultShutdownDeadline = 30 * time.Second

type Service interface {
	// Run executes the provided logic functions in parallel and executes shutdown handlers after
	Run(...func(context.Context) error) error

	// RunContext executes the provided logic functions in parallel with a context and executes shutdown handlers after
	RunContext(context.Context, ...func(context.Context) error) error

	// RegisterShutdownHandler registers a graceful shutdown handler
	RegisterShutdownHandler(...func(context.Context) error)

	// RequestShutdown cancels the run context giving the main logic time to exit
	RequestShutdown()

	// Shutdown cancels the run context and executes graceful shutdown handlers immediately
	Shutdown() error
}

type serviceImpl struct {
	ctx    context.Context
	err    error
	cancel context.CancelFunc

	shutdownHandlers     []func(context.Context) error
	shutdownDeadline     time.Duration
	shutdownOnce         sync.Once
	shutdownHandlerMutex sync.Mutex
}

// NewService builds a new service with the default 30-second shutdown deadline
func NewService() Service {
	return NewServiceWithShutdownDeadline(defaultShutdownDeadline)
}

// NewServiceWithShutdownDeadline builds a new service with a specific shutdown deadline (default if invalid)
func NewServiceWithShutdownDeadline(shutdownDeadline time.Duration) Service {
	if shutdownDeadline <= 0 {
		shutdownDeadline = defaultShutdownDeadline
	}

	return &serviceImpl{shutdownDeadline: shutdownDeadline}
}

func (s *serviceImpl) Run(logic ...func(context.Context) error) error {
	return s.RunContext(context.Background(), logic...)
}

func (s *serviceImpl) RunContext(parentCtx context.Context, logic ...func(context.Context) error) error {
	// create a new context that will be canceled on interrupt signals
	s.ctx, s.cancel = signal.NotifyContext(
		parentCtx,
		os.Interrupt,
		syscall.SIGTERM,
		syscall.SIGQUIT,
	)

	defer s.cancel()

	// execute the provided logic functions in parallel and store the first error
	s.err = processLogic(s.ctx, logic...)

	// execute the shutdown handlers in parallel and join any errors for return
	if err := s.Shutdown(); err != nil {
		if s.err != nil {
			s.err = errors.Join(s.err, err)
		} else {
			s.err = err
		}
	}

	return s.err
}

func (s *serviceImpl) RegisterShutdownHandler(handlers ...func(context.Context) error) {
	s.shutdownHandlerMutex.Lock()
	defer s.shutdownHandlerMutex.Unlock()

	s.shutdownHandlers = append(s.shutdownHandlers, handlers...)
}

func (s *serviceImpl) RequestShutdown() {
	// signal the run logic to exit
	if s.cancel != nil {
		s.cancel()
	}
}

func (s *serviceImpl) Shutdown() error {
	var err error

	// only execute the shutdown handlers once
	s.shutdownOnce.Do(func() {
		// signal the run logic to exit
		if s.cancel != nil {
			s.cancel()
		}

		// process the shutdown handlers with the configured deadline
		s.shutdownHandlerMutex.Lock()
		defer s.shutdownHandlerMutex.Unlock()

		// create a context that will be canceled when the shutdown deadline is reached
		ctx, cancel := context.WithTimeout(context.Background(), s.shutdownDeadline)
		defer cancel()

		// execute the shutdown handlers in parallel and store the first error
		err = processLogic(ctx, s.shutdownHandlers...)
	})

	return err
}

func processLogic(
	incomingCtx context.Context,
	logic ...func(context.Context) error,
) error {
	// create a context that will be canceled when the first error occurs
	ctx, cancel := context.WithCancelCause(incomingCtx)

	// run logic asynchronously
	go func() {
		// cancel the context after all logic has been executed
		defer cancel(nil)

		// run each piece of logic in a goroutine
		eg := &errgroup.Group{}

		for i := range logic {
			fn := logic[i]

			eg.Go(func() error {
				if err := fn(ctx); err != nil {
					// immediately cancel the context and stop processing logic when an error occurs
					cancel(err)
				}

				return nil
			})
		}

		// block until all logic has been executed
		if err := eg.Wait(); err != nil {
			cancel(err)
		}
	}()

	// wait until asynchronous logic is finished (or canceled)
	<-ctx.Done()

	// check for and return errors other than context.Canceled
	if err := context.Cause(ctx); err != nil && !errors.Is(err, context.Canceled) {
		return err
	}

	return nil
}
