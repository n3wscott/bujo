package options

import (
	"github.com/spf13/cobra"
	"time"
)

const (
	layoutISO = "2006-01-02"
)

// AddOptions
type AddOptions struct {
	Message  string
	OnString string
}

func AddEventArgs(cmd *cobra.Command, o *AddOptions) {
	cmd.Flags().StringVar(&o.OnString, "on", "",
		`Specify a date for the event, example: --on="2020-02-28".`)
}

func (o *AddOptions) GetOn() (*time.Time, error) {
	if o.OnString == "" {
		return nil, nil
	}
	t, err := time.Parse(layoutISO, o.OnString)
	if err != nil {
		return nil, err
	}
	return &t, nil
}
