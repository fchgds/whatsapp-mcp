//go:build !windows

package launcher

import "fmt"

type platformState struct{}

func (l *Launcher) startDaemon() error {
	return fmt.Errorf("auto-arranque del daemon no soportado en esta plataforma")
}

func (l *Launcher) closePlatform() error {
	return nil
}
