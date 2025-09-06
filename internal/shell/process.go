package shell

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"sync"
)

// Process is a running command
type Process interface {
	Wait(ctx context.Context) error
	Stop(ctx context.Context) error
	Restart(ctx context.Context) (Process, error)
}

// Wrap a command in a process
func newProcess(cmd *Cmd) *process {
	return &process{
		cmd:    cmd,
		exitCh: make(chan error),
	}
}

var _ Process = (*process)(nil)

type onceError struct {
	o   sync.Once
	err error
}

func (e *onceError) Do(fn func() error) (err error) {
	e.o.Do(func() { e.err = fn() })
	return e.err
}

type process struct {
	cmd    *Cmd
	exitCh chan error
	once   onceError
}

func (p *process) wait() {
	p.exitCh <- p.cmd.exec().Wait()
}

// Stop the process. We first try interrupting. If the context is canceled
// while waiting, we switch to kill.
func (p *process) stop(ctx context.Context) error {
	sp := p.cmd.Process
	if sp == nil {
		return nil
	}

	// Default to interrupt signal
	expectError := isInterrupt
	signal := os.Interrupt

	// Send the signal to the process
	if err := sp.Signal(signal); err != nil {
		if isProcessDone(err) {
			return nil
		}
		// If the signal errored, switch to kill
		expectError = isKilled
		signal = os.Kill
		if err := sp.Signal(signal); err != nil {
			return err
		}
	}

	var err error
	select {
	// Wait for the process to exit
	case err = <-p.exitCh:
	// If the context is canceled, we switch to kill
	case <-ctx.Done():
		return p.kill()
	}

	// Cleanup the exit channel
	close(p.exitCh)

	// If we got an error, check if it's expected or not
	if err != nil && !expectError(err) {
		return err
	}

	return nil
}

// Kill the process. Should only be called once
func (p *process) kill() error {
	sp := p.cmd.Process
	if sp == nil {
		return nil
	}

	// Send a kill signal to the process
	if err := sp.Kill(); err != nil {
		if isProcessDone(err) {
			return nil
		}
		return err
	}

	// Wait for the process to exit
	err := <-p.exitCh
	close(p.exitCh)

	if err != nil && !isKilled(err) {
		return err
	}
	return nil
}

func (p *process) Stop(ctx context.Context) (err error) {
	return p.once.Do(func() error {
		return p.stop(ctx)
	})
}

func (p *process) Kill() (err error) {
	return p.once.Do(p.kill)
}

func (p *process) Wait(ctx context.Context) error {
	select {
	case <-ctx.Done():
		return p.Kill()
	case err := <-p.exitCh:
		return err
	}
}

func (p *process) Restart(ctx context.Context) (Process, error) {
	// Close the process first
	if err := p.Stop(ctx); err != nil {
		return nil, err
	}
	// Re-run the command again. cmd.Args[0] is the path, so we skip that.
	next := exec.Command(p.cmd.Path, p.cmd.Args[1:]...)
	next.Env = p.cmd.Env
	next.Stdout = p.cmd.Stdout
	next.Stderr = p.cmd.Stderr
	next.Stdin = p.cmd.Stdin
	next.ExtraFiles = p.cmd.ExtraFiles
	next.Dir = p.cmd.Dir
	cmd := (*Cmd)(next)
	return cmd.Start()
}

func isProcessDone(err error) bool {
	return errors.Is(err, os.ErrProcessDone)
}

func isInterrupt(err error) bool {
	return err != nil && err.Error() == `signal: interrupt`
}

func isKilled(err error) bool {
	return err != nil && err.Error() == `signal: killed`
}
