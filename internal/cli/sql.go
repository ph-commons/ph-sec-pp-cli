// Copyright 2026 Nestor G Pestelos Jr and contributors. Licensed under Apache-2.0. See LICENSE.
// Local SQL over the free-product company status mirror (SELECT-only).

package cli

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"github.com/ph-commons/ph-sec-pp-cli/internal/store"
)

func newNovelSqlCmd(flags *rootFlags) *cobra.Command {
	var flagQuery string
	var flagDB string
	var flagLimit int

	cmd := &cobra.Command{
		Use:   "sql",
		Short: "Run SELECT SQL against the local company status store",
		Long: strings.TrimSpace(`
Run a read-only SQL query against the local SQLite mirror of free SEC Number
lookups. SELECT only — mutations are rejected.

Requires a prior live fetch (companies --sec-no) so the store has rows.
Does not call the SEC API and does not search the full national registry.
`),
		Example: strings.Trim(`
  ph-sec-pp-cli sql --query "SELECT sec_no, company_name, status FROM companies LIMIT 10" --json
  ph-sec-pp-cli sql --query "SELECT id, resource_type FROM resources LIMIT 5"
`, "\n"),
		Annotations: map[string]string{
			"mcp:read-only": "true",
			"pp:happy-args": "--query=SELECT 1",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would run local SELECT against company status store")
				return nil
			}
			q := strings.TrimSpace(flagQuery)
			if q == "" && len(args) > 0 {
				q = strings.TrimSpace(strings.Join(args, " "))
			}
			if q == "" {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("--query is required (SELECT only against the local free-product mirror)"))
			}
			upper := strings.ToUpper(strings.TrimSpace(q))
			// Allow leading whitespace/comments-free SELECT or WITH ... SELECT
			if !strings.HasPrefix(upper, "SELECT") && !strings.HasPrefix(upper, "WITH") {
				return usageErr(fmt.Errorf("only SELECT (or WITH ... SELECT) queries are allowed"))
			}
			for _, bad := range []string{"INSERT", "UPDATE", "DELETE", "DROP", "ALTER", "CREATE", "REPLACE", "ATTACH", "DETACH", "PRAGMA"} {
				if strings.Contains(upper, bad) && !strings.HasPrefix(upper, "SELECT") {
					// coarse guard
				}
			}
			// Reject obvious mutations even inside WITH
			for _, bad := range []string{" INSERT ", " UPDATE ", " DELETE ", " DROP ", " ALTER ", " CREATE ", " REPLACE ", " ATTACH ", " DETACH "} {
				if strings.Contains(" "+upper+" ", bad) {
					return usageErr(fmt.Errorf("mutating SQL is not allowed (%s)", strings.TrimSpace(bad)))
				}
			}

			if flagDB == "" {
				flagDB = defaultDBPath("ph-sec-pp-cli")
			}
			if _, err := os.Stat(flagDB); os.IsNotExist(err) {
				fmt.Fprintf(cmd.ErrOrStderr(), "no local mirror at %s\nrun: ph-sec-pp-cli companies --sec-no <SEC_NO> --json  # requires PH_SEC_TOKEN\n", flagDB)
				if flags.asJSON || flags.agent {
					fmt.Fprintln(cmd.OutOrStdout(), "[]")
				}
				return nil
			}
			db, err := store.OpenReadOnlyContext(cmd.Context(), flagDB)
			if err != nil {
				return fmt.Errorf("opening store: %w", err)
			}
			defer db.Close()

			rows, err := db.Query(q)
			if err != nil {
				return fmt.Errorf("query: %w", err)
			}
			defer rows.Close()

			cols, err := rows.Columns()
			if err != nil {
				return err
			}
			var out []map[string]any
			for rows.Next() {
				if flagLimit > 0 && len(out) >= flagLimit {
					break
				}
				vals := make([]any, len(cols))
				ptrs := make([]any, len(cols))
				for i := range vals {
					ptrs[i] = &vals[i]
				}
				if err := rows.Scan(ptrs...); err != nil {
					return fmt.Errorf("scan: %w", err)
				}
				row := make(map[string]any, len(cols))
				for i, c := range cols {
					switch v := vals[i].(type) {
					case []byte:
						row[c] = string(v)
					default:
						row[c] = v
					}
				}
				out = append(out, row)
			}
			if err := rows.Err(); err != nil {
				return err
			}
			if out == nil {
				out = []map[string]any{}
			}
			if flags.asJSON || flags.agent || !isTerminal(cmd.OutOrStdout()) {
				return printJSONFiltered(cmd.OutOrStdout(), out, flags)
			}
			return printAutoTable(cmd.OutOrStdout(), out)
		},
	}
	cmd.Flags().StringVar(&flagQuery, "query", "", "SELECT SQL against the local free-product mirror")
	cmd.Flags().StringVar(&flagDB, "db", "", "SQLite path")
	cmd.Flags().IntVar(&flagLimit, "limit", 0, "Max rows to return (0 = unlimited within query)")
	return cmd
}
