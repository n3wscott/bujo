package snake

import (
	"fmt"
	"io/ioutil"
	"strings"

	"github.com/manifoldco/promptui"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

func PromptNext(cmd *cobra.Command, args []string) error {
	fmt.Println("starting with these args:", args)
	subcommands := cmd.Commands()

	templates := &promptui.SelectTemplates{
		Label:    "{{ . }}?",
		Active:   "\u279C  {{ .Name | bold }} {{ .Short | green }}",
		Inactive: "   {{ .Name }} {{ .Short | cyan }}",
		Selected: "{{ .Use | bold }}",
		Details: `
--------- Details ----------
{{ .Long }}
`,
	}

	searcher := func(input string, index int) bool {
		subcommand := subcommands[index]
		name := strings.Replace(strings.ToLower(subcommand.Short), " ", "", -1)
		input = strings.Replace(strings.ToLower(input), " ", "", -1)

		return strings.Contains(name, input)
	}

	prompt := promptui.Select{
		HideHelp:  true,
		Label:     "Commands",
		Items:     subcommands,
		Templates: templates,
		Size:      10,
		Searcher:  searcher,
		Stdin:     ioutil.NopCloser(cmd.InOrStdin()),
		Stdout:    NopCloser(cmd.OutOrStdout()),
	}

	i, _, err := prompt.Run()

	if err != nil {
		fmt.Printf("Prompt failed %v\n", err)
		return err
	}

	fmt.Printf("You choose number %d: %s\n", i+1, subcommands[i].Short)

	next := subcommands[i]

	if next.HasSubCommands() {
		return PromptNext(next, args)
	}

	fmt.Printf("Need a wizard entry now... TODO.\n")
	next.Help()

	if next.Args != nil {
		fmt.Printf("requires some arguments.\n")
	}

	PromptFlags(next, args)

	return nil
}

// PromptFlags propts for flags.
func PromptFlags(cmd *cobra.Command, args []string) error {
	var fs []*pflag.Flag

	if flagset := cmd.Flags(); flagset != nil {
		fmt.Printf("has flags.\n")
		flagset.VisitAll(func(f *pflag.Flag) {
			fs = append(fs, f)
		})
	} else {
		return nil
	}

	fs = append(fs, &pflag.Flag{
		Name:   "Continue...",
		Hidden: true,
		Value:  &continueType{},
	})

	templates := &promptui.SelectTemplates{
		Label:    "{{ . | magenta }} flags?",
		Active:   "\u279C {{ if eq .Value.Type \"continue\" }}{{ .Name | bold | green }}{{ else }}{{ .Name | bold }} {{ .Usage | green | cyan }}{{ end }}",
		Inactive: "  {{ if eq .Value.Type \"continue\" }}{{ .Name | faint | green }}{{ else }}{{ .Name }} {{ .Usage | cyan }}{{ end }}",
		Selected: "{{ if eq .Value.Type \"continue\" }}{{ .Name | bold | green }}{{ else }}{{ .Name | bold }}{{ end }}",
		Details: `
--------- Details ----------
default: {{ .DefValue }}
type: {{ .Value.Type }}
`,
	}

	searcher := func(input string, index int) bool {
		f := fs[index]
		name := strings.Replace(strings.ToLower(f.Name), " ", "", -1)
		input = strings.Replace(strings.ToLower(input), " ", "", -1)

		return strings.Contains(name, input)
	}
	cont := true
	index := 0
	for cont {
		prompt := promptui.Select{
			HideHelp:  true,
			Label:     "Flags",
			Items:     fs,
			Templates: templates,
			Size:      10,
			CursorPos: index,
			Searcher:  searcher,
			Stdin:     ioutil.NopCloser(cmd.InOrStdin()),
			Stdout:    NopCloser(cmd.OutOrStdout()),
		}

		i, _, err := prompt.Run()

		if err != nil {
			fmt.Printf("Prompt failed %v\n", err)
			return err
		}
		index = i

		fmt.Printf("You choose number %d: %s\n", i+1, fs[i].Name)

		more := ""

		switch t := fs[i].Value.Type(); t {
		case "bool":
			cont, more = PromptFlagBool(fs[i])

		// TODO: case int, float.

		case "string":
			cont, more = PromptFlagString(fs[i])

		case "continue":
			cont = false
			more = ""

		default:
			fmt.Printf("%q flag type not yet supported\n", t)
		}

		if more != "" {
			fmt.Println("Append this:", more)
			args = append(args, more)
		}
	}

	args = append(args, "--interactive=false")

	fmt.Println("Run this:", cmd.Use, strings.Join(args, " "))
	cmd.SetArgs(args)
	return cmd.Execute() // This returns back to interactive mode for some reason...
	//return nil
}

type continueType struct{}

func (*continueType) String() string {
	return "continue"
}

func (*continueType) Set(string) error {
	return nil
}

func (*continueType) Type() string {
	return "continue"
}
