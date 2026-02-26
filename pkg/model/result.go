package model

import "time"

type StepStatus string

const (
	StepStatusPlanned    StepStatus = "planned"
	StepStatusSuccess    StepStatus = "success"
	StepStatusFailed     StepStatus = "failed"
	StepStatusSkipped    StepStatus = "skipped"
	StepStatusInProgress StepStatus = "in_progress"
)

type StepResult struct {
	Name     string        `json:"name"`
	Status   StepStatus    `json:"status"`
	Duration time.Duration `json:"duration"`
	Message  string        `json:"message,omitempty"`
}

type BootstrapResult struct {
	Status         string       `json:"status"`
	StartedAt      time.Time    `json:"started_at"`
	EndedAt        time.Time    `json:"ended_at"`
	VMHost         string       `json:"vm_host"`
	VMUser         string       `json:"vm_user"`
	Cluster        string       `json:"cluster"`
	KubeconfigPath string       `json:"kubeconfig_path"`
	DryRun         bool         `json:"dry_run"`
	Steps          []StepResult `json:"steps"`
	Error          string       `json:"error,omitempty"`
}
