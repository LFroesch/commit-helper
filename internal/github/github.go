package github

import (
	"encoding/json"
	"fmt"
	"os/exec"
	"time"
)

type WorkflowRun struct {
	ID         int64     `json:"databaseId"`
	Name       string    `json:"name"`
	Status     string    `json:"status"`
	Conclusion string    `json:"conclusion"`
	Branch     string    `json:"headBranch"`
	Event      string    `json:"event"`
	CreatedAt  time.Time `json:"createdAt"`
	UpdatedAt  time.Time `json:"updatedAt"`
	URL        string    `json:"url"`
}

func IsGHInstalled() bool {
	cmd := exec.Command("gh", "--version")
	return cmd.Run() == nil
}

func IsGHAuthed() bool {
	cmd := exec.Command("gh", "auth", "status")
	return cmd.Run() == nil
}

func GetWorkflowRuns(repoPath string, limit int) ([]WorkflowRun, error) {
	cmd := exec.Command("gh", "run", "list", "--json",
		"databaseId,name,status,conclusion,headBranch,event,createdAt,updatedAt,url",
		"--limit", fmt.Sprintf("%d", limit))
	cmd.Dir = repoPath
	output, err := cmd.Output()
	if err != nil {
		return nil, err
	}
	var runs []WorkflowRun
	if err := json.Unmarshal(output, &runs); err != nil {
		return nil, err
	}
	return runs, nil
}

func GetRunLogs(repoPath string, runID int64) (string, error) {
	cmd := exec.Command("gh", "run", "view", fmt.Sprintf("%d", runID), "--log")
	cmd.Dir = repoPath
	output, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return string(output), nil
}

func RerunWorkflow(repoPath string, runID int64) error {
	cmd := exec.Command("gh", "run", "rerun", fmt.Sprintf("%d", runID))
	cmd.Dir = repoPath
	return cmd.Run()
}
