package commands

import (
	"context"
	"errors"
	"fmt"
	"github.com/n3wscott/bujo/pkg/commands/options"
	"github.com/n3wscott/bujo/pkg/glyph"
	"github.com/n3wscott/bujo/pkg/runner/get"
	"github.com/n3wscott/bujo/pkg/snake"
	"github.com/n3wscott/bujo/pkg/store"
	"github.com/spf13/cobra"
	"os"
	"strings"
)

func addGet_old(topLevel *cobra.Command) {
	co := &options.CollectionOptions{}
	io := &options.IDOptions{}
	i := &options.InteractiveOptions{}

	long := strings.Builder{}
	long.WriteString("Get all or a filtered set of bullets.\n\n")
	long.WriteString("Bullet and aliases:\n")

	validArgs := make([]string, 0, 0)

	for _, g := range glyph.DefaultBullets() {
		if g.Symbol == "" {
			continue
		}

		long.WriteString(fmt.Sprintf("%s: %s\n", g.Symbol, strings.Join(g.Aliases, ", ")))
		if g.Noun != "" {
			validArgs = append(validArgs, g.Noun)
		}
	}

	cmd := &cobra.Command{
		Use:   "get [bullet]",
		Short: "get something",
		Long:  long.String(),
		Example: `
bujo get notes
bujo get tasks --collection future
bujo get completed --all
`,
		Args: func(cmd *cobra.Command, args []string) error {
			if i.Interactive {
				return nil
			}

			if len(args) < 1 {
				co.Bullet = glyph.Any
				return nil
			}
			var err error
			co.Bullet, err = glyph.BulletForAlias(args[0])

			if len(args) > 1 {
				if co.Collection != "today" {
					return errors.New("too many collections set, confused")
				}
				co.Collection = strings.Join(args[1:], " ")
			}

			return err
		},
		ValidArgs: validArgs,
		PreRunE: func(cmd *cobra.Command, args []string) error {
			if i.Interactive {
				return snake.PromptNext(cmd, args)
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			p, err := store.Load(nil)
			if err != nil {
				return err
			}
			s := get.Get{
				ShowID:          io.ShowID,
				Bullet:          co.Bullet,
				Persistence:     p,
				Collection:      co.Collection,
				ListCollections: co.List,
			}
			if co.All {
				s.Collection = ""
			}
			err = s.Do(context.Background())
			return output.HandleError(err)
		},
	}

	options.AddCollectionArgs(cmd, co)
	flagName := "collection"
	_ = cmd.RegisterFlagCompletionFunc(flagName, func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		return collectionCompletions(toComplete), cobra.ShellCompDirectiveNoFileComp
	})

	options.InteractiveArgs(cmd, i)
	options.AddAllCollectionsArg(cmd, co)
	options.AddShowIDArgs(cmd, io)

	topLevel.AddCommand(cmd)
}

func addGet(topLevel *cobra.Command) {
	co := &options.CollectionOptions{}
	io := &options.IDOptions{}
	i := &options.InteractiveOptions{}

	long := strings.Builder{}
	long.WriteString("Get all or a filtered set of bullets.\n\n")
	long.WriteString("Bullet and aliases:\n")

	validArgs := make([]string, 0, 0)

	for _, g := range glyph.DefaultBullets() {
		if g.Symbol == "" {
			continue
		}
		long.WriteString(fmt.Sprintf("%s: %s\n", g.Symbol, strings.Join(g.Aliases, ", ")))
		if g.Noun != "" {
			validArgs = append(validArgs, g.Noun)
		}
	}

	cmd := &cobra.Command{
		Use:   "get [bullet]",
		Short: "get [bullet] --collection todo",
		Long:  long.String(),
		Example: `
bujo get notes
bujo get tasks --collection future
bujo get completed --all
`,
		Args: func(cmd *cobra.Command, args []string) error {
			if i.Interactive {
				return nil
			}

			if len(args) < 1 {
				co.Bullet = glyph.Any
				return nil
			}
			var err error
			co.Bullet, err = glyph.BulletForAlias(args[0])

			if len(args) > 1 {
				if co.Collection != "today" {
					return errors.New("too many collections set, confused")
				}
				co.Collection = strings.Join(args[1:], " ")
			}

			return err
		},
		ValidArgs: validArgs,
		PreRunE: func(cmd *cobra.Command, args []string) error {
			if i.Interactive {
				if err := snake.PromptNext(cmd, args); err != nil {
					return err
				}
				os.Exit(0)
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			p, err := store.Load(nil)
			if err != nil {
				return err
			}
			s := get.Get{
				ShowID:          io.ShowID,
				Bullet:          co.Bullet,
				Persistence:     p,
				Collection:      co.Collection,
				ListCollections: co.List,
			}
			if co.All {
				s.Collection = ""
			}

			fmt.Printf("Going to run %#v\n", s)

			err = s.Do(context.Background())
			return output.HandleError(err)
		},
	}

	options.AddCollectionArgs(cmd, co)
	flagName := "collection"
	_ = cmd.RegisterFlagCompletionFunc(flagName, func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		return collectionCompletions(toComplete), cobra.ShellCompDirectiveNoFileComp
	})

	options.InteractiveArgs(cmd, i)
	options.AddAllCollectionsArg(cmd, co)
	options.AddShowIDArgs(cmd, io)

	for _, g := range glyph.DefaultBullets() {
		//if g.Symbol == "" {
		//	continue
		//}
		addGetBullet(cmd, g)
	}

	topLevel.AddCommand(cmd)
}

func addGetBullet(topLevel *cobra.Command, bullet glyph.Glyph) {
	io := &options.IDOptions{}
	co := &options.CollectionOptions{}

	long := strings.Builder{}
	long.WriteString("Get bullets for " + bullet.String() + ".")

	cmd := &cobra.Command{
		Use:     bullet.Noun,
		Short:   bullet.Noun,
		Aliases: bullet.Aliases,

		Long: fmt.Sprintf("%s (%s), %s\nAliases: %s",
			bullet.Symbol,
			bullet.Noun,
			bullet.Meaning,
			bullet.Aliases),

		RunE: func(cmd *cobra.Command, args []string) error {
			p, err := store.Load(nil)
			if err != nil {
				return err
			}
			s := get.Get{
				ShowID:          io.ShowID,
				Bullet:          co.Bullet,
				Persistence:     p,
				Collection:      co.Collection,
				ListCollections: co.List,
			}
			if co.All {
				s.Collection = ""
			}
			err = s.Do(context.Background())
			return output.HandleError(err)
		},
	}

	options.AddCollectionArgs(cmd, co)
	flagName := "collection"
	_ = cmd.RegisterFlagCompletionFunc(flagName, func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		return collectionCompletions(toComplete), cobra.ShellCompDirectiveNoFileComp
	})

	options.AddShowIDArgs(cmd, io)
	options.AddAllCollectionsArg(cmd, co)

	topLevel.AddCommand(cmd)
}
