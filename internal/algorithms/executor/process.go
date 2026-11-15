package executor

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"
)

// Process owns OS execution only. Domain adapters validate their response contracts.
type Process struct {
	Command     []string
	Directory   string
	Timeout     time.Duration
	OutputLimit int
}
type bounded struct {
	bytes.Buffer
	limit int
}

func (b *bounded) Write(p []byte) (int, error) {
	if b.Len()+len(p) > b.limit {
		return 0, fmt.Errorf("output limit exceeded")
	}
	return b.Buffer.Write(p)
}

func Run(parent context.Context, p Process, body []byte) ([]byte, error) {
	if len(p.Command) == 0 || p.Timeout <= 0 || p.OutputLimit <= 0 {
		return nil, fmt.Errorf("invalid process configuration")
	}
	if len(body) > 16<<20 {
		return nil, fmt.Errorf("input limit exceeded")
	}
	ctx, cancel := context.WithTimeout(parent, p.Timeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, p.Command[0], p.Command[1:]...)
	cmd.Dir = p.Directory
	cmd.Stdin = bytes.NewReader(append(append([]byte(nil), body...), '\n'))
	cmd.WaitDelay = 250 * time.Millisecond
	configureProcess(cmd)
	for _, v := range os.Environ() {
		key, _, _ := strings.Cut(v, "=")
		if !strings.EqualFold(key, "TRAFFIC_DESKTOP_TOKEN") {
			cmd.Env = append(cmd.Env, v)
		}
	}
	out, logs := &bounded{limit: p.OutputLimit}, &bounded{limit: 16 << 10}
	cmd.Stdout = out
	cmd.Stderr = logs
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("worker failed: %w: %.1000s", err, logs.String())
	}
	return append([]byte(nil), out.Bytes()...), nil
}
