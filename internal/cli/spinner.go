// Package cli holds presentation helpers shared by Cobra commands.
package cli

import (
	"fmt"
	"io"
	"os"
	"sync/atomic"
	"time"

	"github.com/mattn/go-isatty"
)

// Spinner renders a single-line "in-progress" indicator on stderr while
// long-running steps run in the background. It auto-disables when stderr
// is not a terminal or when NO_COLOR is set, so piped/CI output stays
// clean.
//
// Lifecycle:
//
//	sp := cli.NewSpinner("starting…")
//	defer sp.Stop()
//	sp.Update("searching…")
//	sp.Update("thinking…")
type Spinner struct {
	out     io.Writer
	enabled bool

	stop  chan struct{}
	done  chan struct{}
	label atomic.Pointer[string]
}

var brailleFrames = []rune{'⠋', '⠙', '⠹', '⠸', '⠼', '⠴', '⠦', '⠧', '⠇', '⠏'}

// NewSpinner starts a spinner with the given initial label. Always call
// Stop() to clear the line and release the goroutine.
func NewSpinner(initial string) *Spinner {
	s := &Spinner{
		out:     os.Stderr,
		enabled: spinnerEnabled(),
		stop:    make(chan struct{}),
		done:    make(chan struct{}),
	}
	label := initial
	s.label.Store(&label)

	if s.enabled {
		go s.run()
	} else {
		close(s.done)
	}
	return s
}

// Update changes the label text shown next to the spinner frame. Safe to
// call from any goroutine. No-op once Stop() has been called.
func (s *Spinner) Update(label string) {
	if !s.enabled {
		return
	}
	s.label.Store(&label)
}

// Stop clears the spinner line and waits for the rendering goroutine to
// finish. Idempotent — safe to call from a defer even if the spinner was
// never enabled.
func (s *Spinner) Stop() {
	if !s.enabled {
		return
	}
	select {
	case <-s.stop:
		// already stopped
		return
	default:
	}
	close(s.stop)
	<-s.done
}

func (s *Spinner) run() {
	defer close(s.done)
	ticker := time.NewTicker(80 * time.Millisecond)
	defer ticker.Stop()

	i := 0
	for {
		select {
		case <-s.stop:
			s.clearLine()
			return
		case <-ticker.C:
			label := *s.label.Load()
			_, _ = fmt.Fprintf(s.out, "\r\033[K\033[2m%c %s\033[0m", brailleFrames[i], label)
			i = (i + 1) % len(brailleFrames)
		}
	}
}

func (s *Spinner) clearLine() {
	_, _ = fmt.Fprint(s.out, "\r\033[K")
}

// spinnerEnabled returns true when stderr is an interactive terminal and
// NO_COLOR is not set. Mirrors fatih/color's logic so spinner and colored
// output stay in sync.
func spinnerEnabled() bool {
	if os.Getenv("NO_COLOR") != "" {
		return false
	}
	fd := os.Stderr.Fd()
	return isatty.IsTerminal(fd) || isatty.IsCygwinTerminal(fd)
}
