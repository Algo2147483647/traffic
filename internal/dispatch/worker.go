package dispatch

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"time"
)

type Manifest struct {
	ID        string   `json:"id"`
	Name      string   `json:"name"`
	Version   int      `json:"contract_version"`
	Command   []string `json:"command"`
	Directory string   `json:"working_directory"`
	Timeout   int      `json:"wall_timeout_ms"`
}
type worker struct{ m Manifest }

func LoadDirectory(dir string) error {
	files, e := filepath.Glob(filepath.Join(dir, "*.json"))
	if e != nil {
		return e
	}
	for _, file := range files {
		data, e := os.ReadFile(file)
		if e != nil {
			return e
		}
		var m Manifest
		if e = json.Unmarshal(data, &m); e != nil {
			return e
		}
		if m.Version != Version || len(m.Command) == 0 || m.Timeout < 50 || m.Timeout > 10000 {
			return fmt.Errorf("invalid worker manifest %s", file)
		}
		if m.Directory == "" {
			m.Directory = "."
		}
		if !filepath.IsAbs(m.Directory) {
			m.Directory = filepath.Join(filepath.Dir(file), m.Directory)
		}
		if m.Name == "" {
			m.Name = m.ID
		}
		if e = Register(Metadata{ID: m.ID, Name: m.Name, Kind: "process"}, func() Policy { return &worker{m} }); e != nil {
			return e
		}
	}
	return nil
}

type bounded struct {
	bytes.Buffer
	limit int
}

func (b *bounded) Write(p []byte) (int, error) {
	if len(p) > b.limit-b.Len() {
		return 0, fmt.Errorf("worker output exceeds limit")
	}
	return b.Buffer.Write(p)
}
func (w *worker) Decide(parent context.Context, in Input) (Proposal, error) {
	ctx, cancel := context.WithTimeout(parent, time.Duration(w.m.Timeout)*time.Millisecond)
	defer cancel()
	body, e := json.Marshal(in)
	if e != nil || len(body) > 16<<20 {
		return Proposal{}, fmt.Errorf("worker input exceeds limit")
	}
	cmd := exec.CommandContext(ctx, w.m.Command[0], w.m.Command[1:]...)
	cmd.Dir = w.m.Directory
	cmd.Stdin = bytes.NewReader(append(body, '\n'))
	cmd.WaitDelay = 250 * time.Millisecond
	configureProcess(cmd)
	out, logs := &bounded{limit: 1 << 20}, &bounded{limit: 16 << 10}
	cmd.Stdout = out
	cmd.Stderr = logs
	// Do not forward desktop authentication secrets to policy processes.
	for _, v := range os.Environ() {
		if !bytes.HasPrefix([]byte(v), []byte("TRAFFIC_DESKTOP_TOKEN=")) {
			cmd.Env = append(cmd.Env, v)
		}
	}
	if e = cmd.Run(); e != nil {
		return Proposal{}, fmt.Errorf("worker failed: %w: %.1000s", e, logs.String())
	}
	var p Proposal
	d := json.NewDecoder(bytes.NewReader(out.Bytes()))
	d.DisallowUnknownFields()
	if e = d.Decode(&p); e != nil {
		return p, e
	}
	var extra any
	if e = d.Decode(&extra); e != io.EOF {
		return p, fmt.Errorf("worker must return one JSON response")
	}
	return p, Validate(in, p)
}
