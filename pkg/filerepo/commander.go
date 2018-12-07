package filerepo

import "os/exec"

// Command provides methods for running subprocesses.
type Command struct{}

// Run starts the specified command and waits for it to complete.
func (*Command) Run(command string, args ...string) error {
	return exec.Command(command, args...).Run() // nolint: gosec
}
