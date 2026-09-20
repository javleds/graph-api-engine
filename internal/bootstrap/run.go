package bootstrap

import (
	"context"
	"errors"
	"net"
	"net/http"
	"sync"
)

func (a *Application) Run(ctx context.Context) error {
	listener, err := net.Listen("tcp", a.Config.HTTP.Address)
	if err != nil {
		return err
	}
	return a.Serve(ctx, listener)
}
func (a *Application) Serve(ctx context.Context, listener net.Listener) error {
	loops, cancelLoops := context.WithCancel(context.Background())
	var background sync.WaitGroup
	background.Add(4)
	go func() { defer background.Done(); a.NodeNotifications.Run(loops) }()
	go func() { defer background.Done(); a.resumeV2(loops) }()
	go func() { defer background.Done(); a.maintenance(loops) }()
	go func() {
		defer background.Done()
		if err := a.Notifications.Run(loops); err != nil {
			a.Logger.Error("notification loop stopped")
			a.ready.Store(false)
		}
	}()
	stopped := make(chan error, 1)
	go func() { stopped <- a.HTTP.Serve(listener) }()
	var serveErr error
	select {
	case <-ctx.Done():
	case serveErr = <-stopped:
	}
	a.ready.Store(false)
	a.Dispatcher.StopAdmission()
	cancelLoops()
	grace, cancel := context.WithTimeout(context.Background(), a.Config.Shutdown.GracePeriod.Value())
	defer cancel()
	if err := a.HTTP.Shutdown(grace); err != nil {
		_ = a.HTTP.Close()
	}
	a.handlerMu.Lock()
	a.handlersClosed = true
	a.handlerMu.Unlock()
	a.handlers.Wait()
	if err := a.Dispatcher.Wait(grace); err != nil {
		a.Dispatcher.Cancel()
		persist, finish := context.WithTimeout(context.Background(), a.Config.Shutdown.PersistenceTimeout.Value())
		if err := a.Dispatcher.Wait(persist); err != nil {
			a.Logger.Error("shutdown persistence deadline exceeded; retaining storage until workers exit")
			_ = a.Dispatcher.Wait(context.Background())
		}
		finish()
	}
	background.Wait()
	if errors.Is(serveErr, http.ErrServerClosed) {
		serveErr = nil
	}
	return serveErr
}
