package snake

import (
	"fmt"
	"github.com/manifoldco/promptui"
	"github.com/spf13/pflag"
	"strconv"
)

func asFlags(f *pflag.Flag) string {
	if f.Shorthand != "" {
		return fmt.Sprintf("--%s, -%s", f.Name, f.Shorthand)
	}
	return fmt.Sprintf("--%s", f.Name)
}

func PromptFlagBool(f *pflag.Flag) (bool, string) {
	fmt.Printf("%s: %s [%s] Default: %s\n", asFlags(f), f.Usage, f.Value.Type(), f.DefValue)

	validInput := "true/false"
	if defTrue, err := ParseBool(f.DefValue); err == nil {
		if defTrue {
			validInput = "[true]/false"
		} else {
			validInput = "true/[false]"
		}
	}

	validate := func(input string) error {
		if input == "" {
			return nil
		}
		_, err := ParseBool(input)
		return err
	}

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

	//fmt.Printf("You answered %s\n", result)

	r, _ := ParseBool(result)

	return true, fmt.Sprintf(`--%s=%t`, f.Name, r)
}

// ParseBool is strconv.ParseBool with the addition of Yes/No parsing.
func ParseBool(str string) (bool, error) {
	switch str {
	case "1", "t", "T", "true", "TRUE", "True", "y", "Y", "yes", "YES", "Yes":
		return true, nil
	case "0", "f", "F", "false", "FALSE", "False", "n", "N", "NO", "No":
		return false, nil
	}
	return false, &strconv.NumError{Func: "ParseBool", Num: str, Err: strconv.ErrSyntax}
}
