//go:build windows

package svc

import (
	"context"
	"log"

	"golang.org/x/sys/windows/svc"
)

// ServiceName is the Windows service name registered in the SCM.
const ServiceName = "SATVOSTallyConnector"

// ConnectorService implements the svc.Handler interface for the Windows
// Service Control Manager. It wraps a context-based run function so the
// rest of the codebase stays platform-agnostic.
type ConnectorService struct {
	RunFunc func(ctx context.Context) error
	cancel  context.CancelFunc
}

// IsWindowsService reports whether the current process is running as a
// Windows service (as opposed to an interactive console session).
func IsWindowsService() bool {
	isService, err := svc.IsWindowsService()
	if err != nil {
		return false
	}
	return isService
}

// Run starts the connector as a Windows service. It blocks until the
// service is stopped via the SCM.
func Run(runFunc func(ctx context.Context) error) error {
	return svc.Run(ServiceName, &ConnectorService{RunFunc: runFunc})
}

// Execute implements the svc.Handler interface.
func (s *ConnectorService) Execute(args []string, r <-chan svc.ChangeRequest, changes chan<- svc.Status) (svcSpecificEC bool, exitCode uint32) {
	const accepted = svc.AcceptStop | svc.AcceptShutdown
	changes <- svc.Status{State: svc.StartPending}

	ctx, cancel := context.WithCancel(context.Background())
	s.cancel = cancel

	// Run the main logic in a goroutine so we can listen for SCM signals.
	errCh := make(chan error, 1)
	go func() {
		errCh <- s.RunFunc(ctx)
	}()

	changes <- svc.Status{State: svc.Running, Accepts: accepted}

	for {
		select {
		case err := <-errCh:
			if err != nil {
				log.Printf("[svc] run function exited with error: %v", err)
				exitCode = 1
			}
			changes <- svc.Status{State: svc.StopPending}
			return
		case c := <-r:
			switch c.Cmd {
			case svc.Interrogate:
				changes <- c.CurrentStatus
			case svc.Stop, svc.Shutdown:
				log.Println("[svc] stop/shutdown requested")
				cancel()
				changes <- svc.Status{State: svc.StopPending}
				// Wait for RunFunc to finish.
				<-errCh
				return
			default:
				log.Printf("[svc] unexpected control request: %d", c.Cmd)
			}
		}
	}
}
