package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
)

// requireKnownSubcommand blocks stray positional args so Cobra can surface
// unknown-command errors (and suggestions) for mistyped subcommands.
func requireKnownSubcommand(cmd *cobra.Command, args []string) error {
	if len(args) == 0 {
		// No args: allow command to run (will print help from Run handler)
		return nil
	}

	// Unknown subcommand: build error message with suggestions
	var suggestion strings.Builder
	suggestion.WriteString("\nDid you mean this?\n")
	if suggestions := cmd.SuggestionsFor(args[0]); len(suggestions) > 0 {
		for _, s := range suggestions {
			suggestion.WriteString("\t" + s + "\n")
		}
	}

	// Print error directly and exit to avoid Cobra's default handling
	fmt.Fprintf(os.Stderr, "Error: unknown command %q for %q%s\nRun 'kwot --help' for usage.\n", args[0], cmd.CommandPath(), suggestion.String())
	os.Exit(1)
	return nil
}

// requireNoArgs validates that no positional arguments are provided.
// Used for leaf subcommands that only accept flags (e.g., kwot apply roles)
func requireNoArgs(cmd *cobra.Command, args []string) error {
	if len(args) == 0 {
		return nil
	}

	// Positional arg provided when only flags are expected
	fmt.Fprintf(os.Stderr, "Error: unknown argument %q for %q\nRun 'kwot %s --help' for usage.\n", args[0], cmd.CommandPath(), cmd.Name())
	os.Exit(1)
	return nil
}
