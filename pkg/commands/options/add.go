package options

import (
	"github.com/spf13/cobra"
)

// AddOptions
type AddOptions struct {
	Message string
}

func AddNoteArgs(cmd *cobra.Command, o *AddOptions) {

}
