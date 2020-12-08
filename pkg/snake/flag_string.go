package snake

import (
	"errors"
	"fmt"
	"github.com/manifoldco/promptui"
	"github.com/spf13/pflag"
)

func PromptFlagString(f *pflag.Flag) (bool, string) {
	fmt.Printf("%s: %s [%s] Default: %s\n", asFlags(f), f.Usage, f.Value.Type(), f.DefValue)

	validate := func(input string) error {
		if len(input) == 0 && len(f.DefValue) == 0 {
			return errors.New("empty")
		}
		return nil
	}

	validInput := fmt.Sprintf(`["%s"]`, f.DefValue)

	templates := &promptui.PromptTemplates{
		Prompt:  "Answer {{ . }} : ",
		Valid:   "Answer {{ . | green }} : ",
		Invalid: "Answer {{ . | red }} : ",
		Success: "{{ . | bold }} : ",
	}

	prompt := promptui.Prompt{
		Label:     validInput,
		Templates: templates,
		Validate:  validate,
	}

	result, err := prompt.Run()

	if err != nil {
		fmt.Printf("Prompt failed %v\n", err)
		return false, ""
	}

	if result == "" {
		result = f.DefValue
	}

	fmt.Printf("You answered %s\n", result)

	return true, fmt.Sprintf(`--%s="%s"`, f.Name, result)
}
