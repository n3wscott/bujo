package commands

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/mitchellh/go-homedir"
	"github.com/spf13/cobra"

	"tableflip.dev/bujo/pkg/app"
	"tableflip.dev/bujo/pkg/store"
)

const (
	apiOutputJSON = "json"
	apiOutputText = "text"
)

type apiOptions struct {
	Output  string
	Journal string
}

type apiConfig struct {
	path string
}

func (c apiConfig) BasePath() string {
	return c.path
}

type apiRuntime struct {
	Journal       string
	JournalSource string
	Persistence   store.Persistence
	Service       *app.Service
}

type apiError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

type apiCodedError struct {
	Code string
	Err  error
}

func (e *apiCodedError) Error() string {
	if e == nil || e.Err == nil {
		return ""
	}
	return e.Err.Error()
}

func (e *apiCodedError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Err
}

func newAPIError(code, msg string) error {
	return &apiCodedError{
		Code: code,
		Err:  errors.New(msg),
	}
}

func wrapAPIError(code string, err error) error {
	if err == nil {
		return nil
	}
	return &apiCodedError{
		Code: code,
		Err:  err,
	}
}

func apiErrorCode(err error) string {
	if err == nil {
		return ""
	}
	var coded *apiCodedError
	if errors.As(err, &coded) && coded.Code != "" {
		return coded.Code
	}
	if errors.Is(err, app.ErrImmutable) {
		return "entry_immutable"
	}
	msg := strings.ToLower(err.Error())
	switch {
	case strings.Contains(msg, "not found"):
		return "entry_not_found"
	case strings.Contains(msg, "no persistence"):
		return "persistence_error"
	default:
		return "internal_error"
	}
}

func validateAPIOutput(output string) error {
	switch output {
	case "", apiOutputJSON, apiOutputText:
		return nil
	default:
		return newAPIError("invalid_argument", fmt.Sprintf("unsupported output %q (expected json or text)", output))
	}
}

func expandUserPath(path string) string {
	trimmed := strings.TrimSpace(path)
	if trimmed == "" {
		return ""
	}
	expanded, err := homedir.Expand(trimmed)
	if err != nil {
		return trimmed
	}
	return expanded
}

func loadAPIRuntime(journalFlag string) (apiRuntime, error) {
	override := expandUserPath(journalFlag)
	var source string
	var cfg store.Config
	if override != "" {
		cfg = apiConfig{path: override}
		source = "flag"
	} else {
		if strings.TrimSpace(os.Getenv("BUJO_PATH")) != "" {
			source = "env"
		} else if strings.TrimSpace(os.Getenv("BUJO_CONFIG_PATH")) != "" {
			source = "config_env"
		} else {
			source = "config_or_default"
		}
		loaded, err := store.LoadConfig()
		if err != nil {
			return apiRuntime{}, wrapAPIError("config_error", err)
		}
		cfg = loaded
	}

	p, err := store.Load(cfg)
	if err != nil {
		return apiRuntime{}, wrapAPIError("persistence_error", err)
	}

	return apiRuntime{
		Journal:       cfg.BasePath(),
		JournalSource: source,
		Persistence:   p,
		Service:       &app.Service{Persistence: p},
	}, nil
}

func apiWrite(cmd *cobra.Command, output string, payload map[string]any) error {
	if output == "" {
		output = apiOutputJSON
	}
	switch output {
	case apiOutputJSON:
		return writeJSON(cmd.OutOrStdout(), payload)
	case apiOutputText:
		return writeText(cmd.OutOrStdout(), payload)
	default:
		// Fall back to JSON to keep machine responses parseable.
		return writeJSON(cmd.OutOrStdout(), payload)
	}
}

func writeJSON(w io.Writer, payload map[string]any) error {
	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(false)
	return enc.Encode(payload)
}

func writeText(w io.Writer, payload map[string]any) error {
	b, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		return err
	}
	_, err = fmt.Fprintln(w, string(b))
	return err
}

func apiSuccess(cmd *cobra.Command, opts *apiOptions, action, journal string, fields map[string]any) error {
	out := map[string]any{
		"ok":      true,
		"action":  action,
		"journal": journal,
	}
	for k, v := range fields {
		out[k] = v
	}
	return apiWrite(cmd, opts.Output, out)
}

func apiFailure(cmd *cobra.Command, opts *apiOptions, action, journal string, err error) error {
	out := map[string]any{
		"ok":      false,
		"action":  action,
		"journal": journal,
		"error": apiError{
			Code:    apiErrorCode(err),
			Message: err.Error(),
		},
	}
	writeErr := apiWrite(cmd, opts.Output, out)
	if writeErr != nil {
		return writeErr
	}
	return err
}

func runAPI(cmd *cobra.Command, opts *apiOptions, action string, fn func(context.Context, apiRuntime) (map[string]any, error)) error {
	if err := validateAPIOutput(opts.Output); err != nil {
		return apiFailure(cmd, opts, action, expandUserPath(opts.Journal), err)
	}

	runtime, err := loadAPIRuntime(opts.Journal)
	if err != nil {
		return apiFailure(cmd, opts, action, expandUserPath(opts.Journal), err)
	}

	fields, err := fn(cmd.Context(), runtime)
	if err != nil {
		return apiFailure(cmd, opts, action, runtime.Journal, err)
	}

	return apiSuccess(cmd, opts, action, runtime.Journal, fields)
}
