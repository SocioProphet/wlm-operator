// Copyright (c) 2019 Sylabs, Inc. All rights reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package slurm

import (
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"

	"github.com/pkg/errors"
)

const (
	// JobStatusCompleted means Slurm job is finished successfully.
	JobStatusCompleted = "COMPLETED"
	// JobStatusCanceled means Slurm job was cancelled.
	JobStatusCanceled = "CANCELLED"
	// JobStatusFailed means job is failed to execute successfully.
	JobStatusFailed = "FAILED"
)

var (
	// ErrInvalidSacctResponse is returned when trying to parse sacct
	// response that is invalid.
	ErrInvalidSacctResponse = errors.New("unable to parse sacct response")
	// ErrFileNotFound is returned when Open fails to find a file.
	ErrFileNotFound = errors.New("file is not found")
)

// JobInfo contains information about a single Slurm job.
type JobInfo struct {
	ID         int            `json:"id" slurm:"JobId"`
	UserID     int            `json:"user_id" slurm:"UserId"`
	Name       string         `json:"name" slurm:"JobName"`
	ExitCode   int            `json:"exit_code" slurm:"ExitCode"`
	State      string         `json:"state" slurm:"JobState"`
	SubmitTime *time.Time     `json:"submit_time" slurm:"SubmitTime"`
	StartTime  *time.Time     `json:"start_time" slurm:"StartTime"`
	RunTime    *time.Duration `json:"run_time" slurm:"RunTime"`
	TimeLimit  *time.Duration `json:"time_limit" slurm:"TimeLimit"`
	WorkDir    string         `json:"work_dir" slurm:"WorkDir"`
	StdOut     string         `json:"std_out" slurm:"StdOut"`
	StdErr     string         `json:"std_err" slurm:"StdErr"`
	Partition  string         `json:"partition" slurm:"Partition"`
	NodeList   string         `json:"node_list" slurm:"NodeList"`
	BatchHost  string         `json:"batch_host" slurm:"BatchHost"`
	NumNodes   int            `json:"num_nodes" slurm:"NumNodes"`
}

// JobStepInfo contains information about Slurm job step.
type JobStepInfo struct {
	ID         string     `json:"id"`
	Name       string     `json:"name"`
	StartedAt  *time.Time `json:"started_at"`
	FinishedAt *time.Time `json:"finished_at"`
	ExitCode   int        `json:"exit_code"`
	State      string     `json:"state"`
}

// Slurm defines interface for interacting with Slurm cluster.
// Interaction can be done in different ways, e.g over ssh, http,
// or even by calling binaries directly.
type Slurm interface {
	// SBatch submits batch job and returns job id if succeeded.
	SBatch(command string) (int64, error)
	// SCancel cancels batch job.
	SCancel(jobID int64) error
	// SJobInfo returns information about a submitted batch job.
	SJobInfo(jobID int64) (*JobInfo, error)
	// SJobSteps returns information about each step in a submitted batch job
	SJobSteps(jobID int64) ([]*JobStepInfo, error)
	// Open opens arbitrary file in a read-only mode on
	// Slurm cluster, e.g. for collecting job results.
	// It is a caller's responsibility to call Close on the returned
	// file to free any allocated resources. Is a file is not found
	// Open will return ErrFileNotFound.
	Open(path string) (io.ReadCloser, error)
	// Tail follows file until close invoked.
	Tail(path string) (io.ReadCloser, error)
}

// ParseSacctResponse is a helper that parses sacct output and
// returns results in a convenient form.
func ParseSacctResponse(raw string) ([]*JobStepInfo, error) {
	lines := strings.Split(strings.Trim(raw, "\n"), "\n")
	infos := make([]*JobStepInfo, len(lines))
	for i, l := range lines {
		splitted := strings.Split(l, "|")
		if len(splitted) != 7 {
			return nil, errors.New("output must contain 6 sections")
		}

		startedAt, err := ParseTime(splitted[0])
		if err != nil {
			return nil, err
		}

		finishedAt, err := ParseTime(splitted[1])
		if err != nil {
			return nil, err
		}

		exitCodeSplitted := strings.Split(splitted[2], ":")
		if len(exitCodeSplitted) != 2 {
			return nil, errors.New("exit code must contain 2 sections")
		}
		exitCode, err := strconv.Atoi(exitCodeSplitted[0])
		if err != nil {
			return nil, err
		}
		j := JobStepInfo{
			StartedAt:  startedAt,
			FinishedAt: finishedAt,
			ExitCode:   exitCode,
			State:      splitted[3],
			ID:         splitted[4],
			Name:       splitted[5],
		}
		infos[i] = &j
	}

	return infos, nil
}

func ParseDuration(durationStr string) (*time.Duration, error) {
	sp := strings.Split(durationStr, ":")
	if len(sp) < 3 {
		// we can skip since data is invalid or not available for that field
		return nil, nil
	}

	if strings.Contains(sp[0], "-") {
		spl := strings.Split(sp[0], "-")
		days, err := strconv.ParseInt(spl[0], 10, 0)
		if err != nil {
			return nil, err
		}

		hours, err := strconv.ParseInt(spl[1], 10, 0)
		if err != nil {
			return nil, err
		}

		sp[0] = strconv.FormatInt(days*24+hours, 10)
	}

	d, err := time.ParseDuration(fmt.Sprintf("%sh%sm%ss", sp[0], sp[1], sp[2]))
	return &d, err
}

func ParseTime(timeStr string) (*time.Time, error) {
	const slurmTimeLayout = "2006-01-02T15:04:05"

	if timeStr == "" || strings.ToLower(timeStr) == "unknown" {
		return nil, nil
	}

	t, err := time.Parse(slurmTimeLayout, timeStr)
	if err != nil {
		return nil, err
	}

	return &t, nil
}
