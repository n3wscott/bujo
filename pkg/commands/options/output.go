package options

import (
	"encoding/json"
	"fmt"
	"github.com/fatih/color"
	"github.com/spf13/cobra"
)

// OutputOptions
type OutputOptions struct {
	JSON bool
}

func AddOutputArg(cmd *cobra.Command, po *OutputOptions) {
	cmd.Flags().BoolVar(&po.JSON, "json", false,
		"Output as JSON.")
}

func (o *OutputOptions) HandleError(err error) error {
	if o.JSON && err != nil {
		out := map[string]string{
			"error": err.Error(),
		}
		b, err := json.Marshal(out)
		if err != nil {
			return err
		}
		_, _ = fmt.Fprintln(color.Output, string(b))
		return nil
	}
	return err
}
