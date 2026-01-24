package commands

import (
	"testing"

	"github.com/spf13/cobra"
)

func TestAddTaskArgsValidation(t *testing.T) {
	root := &cobra.Command{Use: "root"}
	addTask(root)

	var taskCmd *cobra.Command
	for _, cmd := range root.Commands() {
		if cmd.Name() == "task" {
			taskCmd = cmd
			break
		}
	}
	if taskCmd == nil {
		t.Fatalf("expected task command to be registered")
	}

	if err := taskCmd.Args(taskCmd, []string{}); err == nil {
		t.Fatalf("expected args validation to fail on empty args")
	}
	if err := taskCmd.Args(taskCmd, []string{"do", "thing"}); err != nil {
		t.Fatalf("expected args validation to pass, got %v", err)
	}
}
