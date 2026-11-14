package algorithms

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"sync"
	"time"
	"traffic/internal/foundation"
)

type Manifest struct {
	ID               string   `json:"id"`
	Name             string   `json:"name"`
	Capability       string   `json:"capability"`
	AlgorithmVersion string   `json:"algorithm_version"`
	ContractVersion  int      `json:"contract_version"`
	Command          []string `json:"command"`
	Directory        string   `json:"working_directory"`
	Timeout          int      `json:"wall_timeout_ms"`
}
type Request struct {
	Version          int                `json:"contract_version"`
	Algorithm        string             `json:"algorithm"`
	AlgorithmVersion string             `json:"algorithm_version"`
	Capability       string             `json:"capability"`
	Stamp            foundation.Stamp   `json:"stamp"`
	Request          uint64             `json:"request"`
	Tick             uint64             `json:"input_tick"`
	Seed             int64              `json:"seed"`
	Parameters       map[string]float64 `json:"parameters"`
	Payload          json.RawMessage    `json:"payload"`
}
type Response struct {
	Version          int              `json:"contract_version"`
	Algorithm        string           `json:"algorithm"`
	AlgorithmVersion string           `json:"algorithm_version"`
	Capability       string           `json:"capability"`
	Stamp            foundation.Stamp `json:"stamp"`
	Request          uint64           `json:"request"`
	Tick             uint64           `json:"input_tick"`
	Payload          json.RawMessage  `json:"payload"`
}

var registry = struct {
	sync.RWMutex
	items map[string]Manifest
}{items: map[string]Manifest{}}

func Register(m Manifest) error {
	if m.ID == "" || m.AlgorithmVersion == "" || m.ContractVersion != 1 || (m.Capability != "route" && m.Capability != "signal") || len(m.Command) == 0 || m.Timeout < 50 || m.Timeout > 10000 {
		return fmt.Errorf("invalid algorithm manifest")
	}
	registry.Lock()
	defer registry.Unlock()
	if _, ok := registry.items[m.ID]; ok {
		return fmt.Errorf("duplicate algorithm %s", m.ID)
	}
	registry.items[m.ID] = m
	return nil
}
func Lookup(id string) (Manifest, bool) {
	registry.RLock()
	defer registry.RUnlock()
	m, ok := registry.items[id]
	return m, ok
}
func List() []Manifest {
	registry.RLock()
	defer registry.RUnlock()
	out := []Manifest{}
	for _, m := range registry.items {
		out = append(out, m)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}
func LoadDirectory(dir string) error {
	files, e := filepath.Glob(filepath.Join(dir, "*.json"))
	if e != nil {
		return e
	}
	for _, f := range files {
		b, e := os.ReadFile(f)
		if e != nil {
			return e
		}
		var m Manifest
		if e = json.Unmarshal(b, &m); e != nil {
			return e
		}
		if m.Directory == "" {
			m.Directory = "."
		}
		if !filepath.IsAbs(m.Directory) {
			m.Directory = filepath.Join(filepath.Dir(f), m.Directory)
		}
		if e = Register(m); e != nil {
			return fmt.Errorf("%s: %w", f, e)
		}
	}
	return nil
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
func Decode(data []byte, v any) error {
	d := json.NewDecoder(bytes.NewReader(data))
	d.DisallowUnknownFields()
	if e := d.Decode(v); e != nil {
		return e
	}
	var extra any
	if e := d.Decode(&extra); e != io.EOF {
		return fmt.Errorf("expected exactly one JSON value")
	}
	return nil
}
func Invoke(parent context.Context, in Request) (Response, error) {
	m, ok := Lookup(in.Algorithm)
	if !ok || m.Capability != in.Capability {
		return Response{}, fmt.Errorf("algorithm unavailable")
	}
	ctx, cancel := context.WithTimeout(parent, time.Duration(m.Timeout)*time.Millisecond)
	defer cancel()
	data, e := json.Marshal(in)
	if e != nil || len(data) > 16<<20 {
		return Response{}, fmt.Errorf("input limit exceeded")
	}
	cmd := exec.CommandContext(ctx, m.Command[0], m.Command[1:]...)
	cmd.Dir = m.Directory
	cmd.Stdin = bytes.NewReader(append(data, '\n'))
	cmd.WaitDelay = 250 * time.Millisecond
	configureProcess(cmd)
	for _, v := range os.Environ() {
		if !bytes.HasPrefix([]byte(v), []byte("TRAFFIC_DESKTOP_TOKEN=")) {
			cmd.Env = append(cmd.Env, v)
		}
	}
	out, logs := &bounded{limit: 4 << 20}, &bounded{limit: 16 << 10}
	cmd.Stdout = out
	cmd.Stderr = logs
	if e = cmd.Run(); e != nil {
		return Response{}, fmt.Errorf("worker: %w: %.400s", e, logs.String())
	}
	var r Response
	if e = Decode(out.Bytes(), &r); e != nil {
		return r, e
	}
	if r.Version != in.Version || r.Algorithm != in.Algorithm || r.AlgorithmVersion != in.AlgorithmVersion || r.Capability != in.Capability || r.Stamp != in.Stamp || r.Tick != in.Tick || r.Request != in.Request {
		return r, fmt.Errorf("mismatched result envelope")
	}
	return r, nil
}
