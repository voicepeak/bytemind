package runtime

import (
	"encoding/json"
	"strings"
)

type TaskReport struct {
	Executed               []string `json:"executed,omitempty"`
	Denied                 []string `json:"denied,omitempty"`
	PendingApproval        []string `json:"pending_approval,omitempty"`
	SkippedDueToDependency []string `json:"skipped_due_to_dependency,omitempty"`
}

func (r *TaskReport) RecordExecuted(name string) {
	r.appendUnique(&r.Executed, name)
}

func (r *TaskReport) RecordDenied(name string) {
	r.appendUnique(&r.Denied, name)
}

func (r *TaskReport) RecordPendingApproval(name string) {
	r.appendUnique(&r.PendingApproval, name)
}

func (r *TaskReport) RecordSkippedDueToDependency(name string) {
	r.appendUnique(&r.SkippedDueToDependency, name)
}

func (r TaskReport) IsEmpty() bool {
	return len(r.Executed) == 0 &&
		len(r.Denied) == 0 &&
		len(r.PendingApproval) == 0 &&
		len(r.SkippedDueToDependency) == 0
}

func (r TaskReport) JSON() string {
	payload, err := json.Marshal(r)
	if err != nil {
		return "{}"
	}
	return string(payload)
}

func (r *TaskReport) appendUnique(target *[]string, name string) {
	if r == nil || target == nil {
		return
	}
	name = strings.TrimSpace(name)
	if name == "" {
		return
	}
	for _, existing := range *target {
		if existing == name {
			return
		}
	}
	*target = append(*target, name)
}
