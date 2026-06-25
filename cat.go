package main

import (
	"fmt"

	"github.com/spf13/cobra"
)

// newCatCmd prints the current project's spec to stdout, scoped to unfinished
// work by default. Read-pure (V16): re-renders from the db via renderSpecScoped,
// never reads or writes SPEC.md. Default selector = openFeatures (durables +
// every unfinished feature, V75); --feature N narrows to one feature and errors
// on an unknown ord (exit != 0). Durables (I,R,V,B) always render in full (V76).
func newCatCmd() *cobra.Command {
	var feature int
	c := &cobra.Command{
		Use:   "cat",
		Short: "print the spec to stdout (durables + unfinished features); read-only",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			db, pid, _, err := openProjectContext()
			if err != nil {
				return err
			}
			defer db.Close()

			sel := openFeatures
			if cmd.Flags().Changed("feature") {
				sel = featureByOrd(int64(feature))
			}
			out, err := renderSpecScoped(db, pid, sel)
			if err != nil {
				return err
			}
			fmt.Fprint(cmd.OutOrStdout(), out)
			return nil
		},
	}
	c.Flags().IntVar(&feature, "feature", 0, "render only this feature ordinal (default: all unfinished)")
	return c
}
