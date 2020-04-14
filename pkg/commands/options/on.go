package options

import (
	"github.com/spf13/cobra"
	"time"
)

const (
	layoutISO      = "2006-1-2"
	layoutISOShort = "1/2"
)

// AddOn
type OnOptions struct {
	OnString string
}

func AddOnArgs(cmd *cobra.Command, o *OnOptions) {
	cmd.Flags().StringVar(&o.OnString, "on", "",
		`Specify a date, example: --on="2020-2-28" or --on="2/28".`)
}

func (o *OnOptions) GetOn() (*time.Time, error) {
	if o.OnString == "" {
		return nil, nil
	}
	t, err := time.Parse(layoutISO, o.OnString)
	if err != nil {
		// Let the year be the same.
		t, err = time.Parse(layoutISOShort, o.OnString)
		if err != nil {
			return nil, err
		}
		t = t.AddDate(time.Now().Year(), 0, 0)
		// I am gonna assume if you said 1/3 on 12/5, you meant next year, not 11 months ago.
		if t.Before(time.Now()) {
			t = t.AddDate(1, 0, 0)
		}
	}
	return &t, nil
}
