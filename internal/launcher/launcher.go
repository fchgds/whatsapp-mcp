package launcher

import (
	"context"
	"fmt"
	"os/exec"
	"sync"
	"time"

	"whatsapp-mcp/internal/ipc"
)

const linkMessage = "necesita QR: mira la ventana que se abrio y escanea desde WhatsApp -> Dispositivos vinculados"

type Launcher struct {
	DaemonPath string
	WorkDir    string
	Client     *ipc.Client

	mu       sync.Mutex
	cmd      *exec.Cmd
	started  bool
	platform platformState
}

func (l *Launcher) EnsureRunning(ctx context.Context) (ipc.Status, error) {
	if l.Client == nil {
		return ipc.Status{}, fmt.Errorf("daemon no configurado")
	}
	if st, err := l.statusWithTimeout(ctx, 800*time.Millisecond); err == nil {
		return st, nil
	}

	l.mu.Lock()
	defer l.mu.Unlock()

	if st, err := l.statusWithTimeout(ctx, 800*time.Millisecond); err == nil {
		return st, nil
	}
	if !l.started {
		if err := l.startDaemon(); err != nil {
			return ipc.Status{}, err
		}
		l.started = true
	}

	deadline := time.Now().Add(20 * time.Second)
	for {
		st, err := l.statusWithTimeout(ctx, 1*time.Second)
		if err == nil && (st.Connected || st.NeedsQR) {
			return st, nil
		}
		if time.Now().After(deadline) {
			if err != nil {
				return ipc.Status{}, fmt.Errorf("no pude arrancar el daemon: %w", err)
			}
			return ipc.Status{}, fmt.Errorf("no pude arrancar el daemon")
		}
		select {
		case <-ctx.Done():
			return ipc.Status{}, ctx.Err()
		case <-time.After(500 * time.Millisecond):
		}
	}
}

func (l *Launcher) Close() error {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.closePlatform()
}

func (l *Launcher) statusWithTimeout(ctx context.Context, timeout time.Duration) (ipc.Status, error) {
	statusCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	return l.Client.Status(statusCtx)
}

func LinkingMessage() string {
	return linkMessage
}
