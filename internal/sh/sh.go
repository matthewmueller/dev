package sh

import (
	"context"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"syscall"
)

type Command struct {
	Stderr io.Writer
	Stdout io.Writer
	Stdin  io.Reader
	Env    []string
	Dir    string

	// Filled in during execution
	cmd  *exec.Cmd
	pgid int
}

func (c *Command) command(ctx context.Context, name string, args ...string) *exec.Cmd {
	// create our command
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	cmd.Stderr = c.Stderr
	cmd.Stdout = c.Stdout
	cmd.Env = c.Env
	cmd.Stdin = c.Stdin
	cmd.Dir = c.Dir
	return cmd
}

func (c *Command) Start(ctx context.Context, name string, args ...string) (err error) {
	c.cmd = c.command(ctx, name, args...)
	if err := c.cmd.Start(); err != nil {
		return err
	}
	c.pgid, err = syscall.Getpgid(c.cmd.Process.Pid)
	if err != nil {
		return err
	}
	// pgid no longer gets killed on sigint
	// so we need to kill it manually ourselves
	signals := make(chan os.Signal, 1)
	signal.Notify(signals, os.Interrupt)
	go func() {
		<-signals
		syscall.Kill(-c.pgid, syscall.SIGKILL)
		os.Exit(1)
	}()
	return nil
}

func (c *Command) Restart(ctx context.Context) error {
	// kill the started process
	if c.pgid != 0 {
		syscall.Kill(-c.pgid, syscall.SIGKILL)
	}
	// Wait for the command to finish if we have one
	if c.cmd != nil {
		c.cmd.Wait()
	}
	// Start the command again
	return c.Start(ctx, c.cmd.Path, c.cmd.Args[1:]...)
}
