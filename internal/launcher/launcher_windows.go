//go:build windows

package launcher

import (
	"fmt"
	"os/exec"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

type platformState struct {
	job windows.Handle
}

func (l *Launcher) startDaemon() error {
	if l.DaemonPath == "" {
		return fmt.Errorf("daemon_path vacio")
	}
	if l.platform.job != 0 {
		windows.CloseHandle(l.platform.job)
		l.platform.job = 0
	}
	job, err := windows.CreateJobObject(nil, nil)
	if err != nil {
		return fmt.Errorf("crear job object: %w", err)
	}
	info := windows.JOBOBJECT_EXTENDED_LIMIT_INFORMATION{}
	info.BasicLimitInformation.LimitFlags = windows.JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE
	if _, err := windows.SetInformationJobObject(
		job,
		windows.JobObjectExtendedLimitInformation,
		uintptr(unsafe.Pointer(&info)),
		uint32(unsafe.Sizeof(info)),
	); err != nil {
		windows.CloseHandle(job)
		return fmt.Errorf("configurar job object: %w", err)
	}

	cmd := exec.Command(l.DaemonPath)
	cmd.Dir = l.WorkDir
	cmd.SysProcAttr = &syscall.SysProcAttr{CreationFlags: windows.CREATE_NEW_CONSOLE}
	if err := cmd.Start(); err != nil {
		windows.CloseHandle(job)
		return fmt.Errorf("arrancar daemon: %w", err)
	}

	proc, err := windows.OpenProcess(windows.PROCESS_SET_QUOTA|windows.PROCESS_TERMINATE, false, uint32(cmd.Process.Pid))
	if err != nil {
		_ = cmd.Process.Kill()
		windows.CloseHandle(job)
		return fmt.Errorf("abrir proceso del daemon: %w", err)
	}
	if err := windows.AssignProcessToJobObject(job, proc); err != nil {
		windows.CloseHandle(proc)
		_ = cmd.Process.Kill()
		windows.CloseHandle(job)
		return fmt.Errorf("asignar daemon al job object: %w", err)
	}
	windows.CloseHandle(proc)

	l.cmd = cmd
	l.platform.job = job
	go func() {
		_ = cmd.Wait()
		l.mu.Lock()
		defer l.mu.Unlock()
		if l.cmd == cmd {
			l.cmd = nil
			l.started = false
			if l.platform.job == job {
				windows.CloseHandle(l.platform.job)
				l.platform.job = 0
			}
		}
	}()
	return nil
}

func (l *Launcher) closePlatform() error {
	if l.platform.job == 0 {
		return nil
	}
	err := windows.CloseHandle(l.platform.job)
	l.platform.job = 0
	l.started = false
	return err
}
