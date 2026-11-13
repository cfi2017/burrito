package base

import (
	"errors"
	"fmt"
	"os"
	"os/exec"

	c "github.com/padok-team/burrito/internal/utils/cmd"
)

// BaseTool provides common functionality for Terraform and OpenTofu
type BaseTool struct {
	ExecPath   string
	WorkingDir string
	ToolName   string // "terraform" or "tofu"
}

func (t *BaseTool) TenvName() string {
	return t.ToolName
}

func (t *BaseTool) Init(workingDir string) error {
	t.WorkingDir = workingDir
	cmd := exec.Command(t.ExecPath, "init", "-no-color", "-upgrade")
	c.Verbose(cmd)
	cmd.Dir = workingDir
	if err := cmd.Run(); err != nil {
		return err
	}
	return nil
}

func (t *BaseTool) Plan(planArtifactPath string) error {
	cmd := exec.Command(t.ExecPath, "plan", "-no-color", "-out", planArtifactPath)
	c.Verbose(cmd)
	cmd.Dir = t.WorkingDir
	if err := cmd.Run(); err != nil {
		return err
	}
	return nil
}

func (t *BaseTool) Apply(planArtifactPath string) error {
	var cmd *exec.Cmd
	if planArtifactPath != "" {
		cmd = exec.Command(t.ExecPath, "apply", "-no-color", "-auto-approve", planArtifactPath)
	} else {
		cmd = exec.Command(t.ExecPath, "apply", "-no-color", "-auto-approve")
	}
	c.Verbose(cmd)
	cmd.Dir = t.WorkingDir
	if err := cmd.Run(); err != nil {
		return err
	}
	return nil
}

func (t *BaseTool) Show(planArtifactPath, mode string) ([]byte, error) {
	var cmd *exec.Cmd
	switch mode {
	case "json":
		cmd = exec.Command(t.ExecPath, "show", "-no-color", "-json", planArtifactPath)
	case "pretty":
		cmd = exec.Command(t.ExecPath, "show", "-no-color", planArtifactPath)
	default:
		return nil, errors.New("invalid mode")
	}

	cmd.Dir = t.WorkingDir
	out, err := cmd.Output()
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (t *BaseTool) StatePull(workingDir string) ([]byte, error) {
	t.WorkingDir = workingDir
	cmd := exec.Command(t.ExecPath, "state", "pull")
	cmd.Dir = t.WorkingDir
	out, err := cmd.Output()
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (t *BaseTool) StatePush(workingDir string, state []byte) error {
	stateFile, err := os.CreateTemp("", "burrito-state-*.tfstate")
	if err != nil {
		return fmt.Errorf("create temporary state file: %w", err)
	}
	statePath := stateFile.Name()
	if _, err := stateFile.Write(state); err != nil {
		closeErr := stateFile.Close()
		removeErr := os.Remove(statePath)
		return errors.Join(fmt.Errorf("write temporary state file: %w", err), closeErr, removeErr)
	}
	if err := stateFile.Close(); err != nil {
		removeErr := os.Remove(statePath)
		return errors.Join(fmt.Errorf("close temporary state file: %w", err), removeErr)
	}
	cmd := exec.Command(t.ExecPath, "state", "push", statePath)
	cmd.Dir = workingDir
	if err := cmd.Run(); err != nil {
		removeErr := os.Remove(statePath)
		return errors.Join(fmt.Errorf("push datastore state into local backend: %w", err), removeErr)
	}
	if err := os.Remove(statePath); err != nil {
		return fmt.Errorf("remove temporary state file: %w", err)
	}
	return nil
}

func (t *BaseTool) GetExecPath() string {
	return t.ExecPath
}
