package commands

import (
	"fmt"
	"io/ioutil"
	"strconv"
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
		Active:   "\u279C  {{ .Name }} {{ .Short | cyan }}",
		Inactive: "   {{ .Name }} {{ .Short | cyan }}",
		Selected: "\u279C  {{ .Use }} {{ .Short | red | cyan }}",
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

	PromptFlags(next)

	return nil
}

// PromptFlags propts for flags.
func PromptFlags(cmd *cobra.Command) error {
	var fs []*pflag.Flag

	if flagset := cmd.Flags(); flagset != nil {
		fmt.Printf("has flags.\n")
		flagset.VisitAll(func(f *pflag.Flag) {
			fs = append(fs, f)
		})
	} else {
		return nil
	}

	templates := &promptui.SelectTemplates{
		Label:    "{{ . | magenta }} flags?",
		Active:   "\u279C  {{ .Name }} {{ .Usage | cyan }}",
		Inactive: "   {{ .Name }} {{ .Usage | cyan }}",
		Selected: "\u279C  {{ .Use }} {{ .Usage | red | cyan }}",
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
	for cont {
		prompt := promptui.Select{
			HideHelp:  true,
			Label:     "Flags",
			Items:     fs,
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

		fmt.Printf("You choose number %d: %s\n", i+1, fs[i].Name)
		cont = demoEdit()
	}

	return nil
}

type pepper struct {
	Name     string
	HeatUnit int
	Peppers  int
}

func demoEdit() bool {
	label := map[string]string{"Foo": "foo bar baz"}

	validate := func(input string) error {
		_, err := strconv.ParseFloat(input, 64)
		if err != nil {
			label["Foo"] = "foo bar baz " + input
		}
		return err
	}

	templates := &promptui.PromptTemplates{
		Prompt:  "{{ .Foo }} \n",
		Valid:   "{{ . | green }} ",
		Invalid: "{{ . | red }} ",
		Success: "{{ . | bold }} ",
	}

	prompt := promptui.Prompt{
		Label:     label,
		Templates: templates,
		Validate:  validate,
	}

	result, err := prompt.Run()

	if err != nil {
		fmt.Printf("Prompt failed %v\n", err)
		return false
	}

	fmt.Printf("You answered %s\n", result)
	return true
}
