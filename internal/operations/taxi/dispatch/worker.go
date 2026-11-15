package dispatch

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
	"traffic/internal/algorithms"
	"traffic/internal/algorithms/executor"
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

func (w *worker) Decide(parent context.Context, in Input) (Proposal, error) {
	body, err := json.Marshal(in)
	if err != nil {
		return Proposal{}, err
	}
	output, err := executor.Run(parent, executor.Process{Command: w.m.Command, Directory: w.m.Directory, Timeout: time.Duration(w.m.Timeout) * time.Millisecond, OutputLimit: 1 << 20}, body)
	if err != nil {
		return Proposal{}, err
	}
	var p Proposal
	if err = algorithms.Decode(output, &p); err != nil {
		return p, err
	}
	return p, Validate(in, p)
}
