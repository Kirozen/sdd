package sdd

import (
	"context"
	"fmt"
	"strings"

	dbq "github.com/kirozen/sdd/internal/db"
	"github.com/spf13/cobra"
)

// retractInterface hard-deletes a durable interface by name (V20). It pre-checks
// the citing tasks (task_cites_iface, a NO ACTION FK) and refuses with the citer
// list BEFORE the DELETE, so the user sees an actionable message, never a raw FK
// error (V95/V5). Re-exports (V8).
func retractInterface(db dbq.DBTX, projectID int64, name string) (string, error) {
	ctx := context.Background()
	q := dbq.New(db)

	iid, err := q.InterfaceIDByName(ctx, dbq.InterfaceIDByNameParams{ProjectID: projectID, Name: name})
	if err != nil {
		return "", fmt.Errorf("no interface I.%s in this project", name)
	}

	taskOrds, err := q.CitersOfIface(ctx, iid)
	if err != nil {
		return "", err
	}
	if len(taskOrds) > 0 {
		var cited []string
		for _, o := range taskOrds {
			cited = append(cited, fmt.Sprintf("T%d", o))
		}
		return "", fmt.Errorf("cannot retract I.%s: cited by %s — retract those first", name, strings.Join(cited, ", "))
	}

	if _, err := q.DeleteInterfaceByName(ctx, dbq.DeleteInterfaceByNameParams{ProjectID: projectID, Name: name}); err != nil {
		return "", err
	}
	return fmt.Sprintf("retracted I.%s", name), nil
}

func newRetractInterfaceCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "retract-interface I.<name>",
		Short: "hard-delete an interface; refuses (listing citers) if cited; re-export",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := strings.TrimPrefix(args[0], "I.")
			return runMutation(func(db dbq.DBTX, pid int64) (string, error) {
				return retractInterface(db, pid, name)
			})
		},
	}
}
