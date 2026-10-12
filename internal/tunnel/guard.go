package tunnel

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"runtime"
	"time"
)

// GuardArg is the first argument of the guard process. The program starts
// itself with it, so no second executable has to be shipped, and whatever
// handles arguments first hands over to RunGuard when it sees it.
const GuardArg = "--moodle-tunnel-guard"

// A guard exists because a process cannot clean up after itself when it is
// killed, and a tunnel left running keeps publishing its address to whatever is
// later given the port it pointed at.
//
// The guard stands between the host and cloudflared. The host holds the write
// end of the guard's standard input, and the operating system closes that end
// when the host exits for any reason, including being killed. The guard reads
// until it ends and then stops cloudflared. It works the same on every
// platform, which a kernel feature for this — there are one or two, and not on
// every system — would not.

// RunGuard runs cloudflared and stops it when its standard input ends or ctx is
// done. args are the grace period to allow cloudflared after an interrupt, as a
// duration, then cloudflared's path and its arguments. It returns the exit code
// the guard process should end with.
func RunGuard(ctx context.Context, args []string) int {
	if len(args) < 2 {
		fmt.Fprintln(os.Stderr, "moodle: the tunnel guard was started without a command")
		return 2
	}
	grace, err := time.ParseDuration(args[0])
	if err != nil {
		fmt.Fprintf(os.Stderr, "moodle: the tunnel guard was given a bad grace period: %v\n", err)
		return 2
	}

	// Where the kernel stops a child when its parent dies, "parent" is the
	// thread that started it. This process does nothing else, so holding its
	// thread for the whole run is what makes that mean "the guard died".
	runtime.LockOSThread()

	cmd := exec.Command(args[1], args[2:]...)
	// cloudflared's output is the host's to read, so it goes straight through.
	cmd.Stdout, cmd.Stderr = os.Stdout, os.Stderr
	dieWithParent(cmd)
	if err := cmd.Start(); err != nil {
		fmt.Fprintf(os.Stderr, "moodle: cannot start %s: %v\n", args[1], err)
		return 1
	}
	finished := make(chan error, 1)
	go func() { finished <- cmd.Wait() }()

	hostGone := make(chan struct{})
	go func() {
		_, _ = io.Copy(io.Discard, os.Stdin)
		close(hostGone)
	}()

	select {
	case err := <-finished:
		return exitCode(err)
	case <-hostGone:
	case <-ctx.Done():
	}
	stopProcess(cmd.Process, grace)
	return exitCode(<-finished)
}

// stopProcess interrupts a process, and kills it if it has not gone by the end
// of the grace period. An interrupt lets cloudflared close its connection to
// the edge; where one cannot be sent, or is ignored, it is killed.
func stopProcess(process *os.Process, grace time.Duration) {
	if err := process.Signal(os.Interrupt); err != nil {
		_ = process.Kill()
		return
	}
	time.AfterFunc(grace, func() { _ = process.Kill() })
}

func exitCode(err error) int {
	if err == nil {
		return 0
	}
	var exit *exec.ExitError
	if errors.As(err, &exit) {
		return exit.ExitCode()
	}
	return 1
}

// selfPath is the program to start as the guard: this one. A binary replaced
// on disk while it runs — an upgrade — is still reachable through the kernel on
// Linux, where the path it was started from no longer is.
func selfPath() (string, error) {
	if path, err := os.Executable(); err == nil {
		if _, err := os.Stat(path); err == nil {
			return path, nil
		}
	}
	if runtime.GOOS == "linux" {
		return "/proc/self/exe", nil
	}
	return "", errors.New("cannot find the running program")
}
