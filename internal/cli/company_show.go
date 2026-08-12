// Copyright 2026 Nestor G Pestelos Jr and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-authored novel command: cache-first company status by SEC number (free product only).

package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"github.com/ph-commons/ph-sec-pp-cli/internal/store"
)

// companyShowView is the user-facing payload for company show.
type companyShowView struct {
	SecNo        string          `json:"sec_no"`
	CompanyName  string          `json:"company_name,omitempty"`
	DateApproved string          `json:"date_approved,omitempty"`
	Licenses     string          `json:"licenses,omitempty"`
	Status       string          `json:"status,omitempty"`
	Source       string          `json:"source"` // local | live
	Note         string          `json:"note,omitempty"`
	Raw          json.RawMessage `json:"raw,omitempty"`
}

func newNovelCompanyShowCmd(flags *rootFlags) *cobra.Command {
	var flagSecNo string
	var flagRefresh bool
	var flagDB string

	cmd := &cobra.Command{
		Use:   "show",
		Short: "Show registration status for a known SEC number (cache-first; free product only)",
		Long: strings.TrimSpace(`
Show registration status for a known Philippine SEC registration number.

This is the free Marketplace SEC Number product only.
- You must already know the SEC number (no name search).
- Live refresh requires PH_SEC_TOKEN (OAuth bearer from Free package).
- Name search, GIS, AFS, and addresses are NOT in this CLI (paid CIL).

Default: read from local SQLite. Use --refresh to call the live free API
(counts against free daily quota, roughly 10 calls/day).
`),
		Example: strings.Trim(`
  ph-sec-pp-cli company show --sec-no A199600179 --json
  ph-sec-pp-cli company show --sec-no A199600179 --refresh --json
`, "\n"),
		Annotations: map[string]string{
			"mcp:read-only": "true",
			"pp:happy-args": "--sec-no=A199600179",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would show company status for sec_no (cache-first; free product only)")
				return nil
			}
			if flagSecNo == "" {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("--sec-no is required (you must already know the SEC number; this CLI has no name search)"))
			}
			secNo := strings.TrimSpace(flagSecNo)

			if flagDB == "" {
				flagDB = defaultDBPath("ph-sec-pp-cli")
			}

			// Prefer local cache unless --refresh.
			if !flagRefresh {
				if _, err := os.Stat(flagDB); os.IsNotExist(err) {
					fmt.Fprintf(cmd.ErrOrStderr(), "no local mirror at %s\nrun: ph-sec-pp-cli companies --sec-no %s --json  # requires PH_SEC_TOKEN; free product only\nthen re-run company show, or pass --refresh once\n", flagDB, secNo)
					if flags.asJSON || flags.agent {
						return printJSONFiltered(cmd.OutOrStdout(), companyShowView{
							SecNo:  secNo,
							Source: "local",
							Note:   "no local mirror; set PH_SEC_TOKEN and run companies --sec-no once, or pass --refresh",
						}, flags)
					}
					return nil
				}
				db, err := store.OpenReadOnlyContext(cmd.Context(), flagDB)
				if err != nil {
					return fmt.Errorf("opening local store: %w", err)
				}
				defer db.Close()

				view, ok, err := loadCompanyShowFromStore(cmd, db, secNo)
				if err != nil {
					return err
				}
				if ok {
					return emitCompanyShow(cmd, flags, view)
				}
				fmt.Fprintf(cmd.ErrOrStderr(), "no cached row for sec_no=%s\nuse --refresh to call the free live API (requires PH_SEC_TOKEN; uses free quota)\n", secNo)
				if flags.asJSON || flags.agent {
					return printJSONFiltered(cmd.OutOrStdout(), companyShowView{
						SecNo:  secNo,
						Source: "local",
						Note:   "not in local cache; pass --refresh with PH_SEC_TOKEN for one free-product live call",
					}, flags)
				}
				return nil
			}

			// Live free-product path.
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			params := map[string]string{"sec_no": secNo}
			data, prov, err := resolveReadWithStrategyAndResponsePath(
				cmd.Context(), c, flags, "live", "companies", false,
				"/client_sec_no_status.php", params, nil, "data", cmd.ErrOrStderr(),
			)
			if err != nil {
				return classifyAPIError(err, flags)
			}
			view := companyShowView{
				SecNo:  secNo,
				Source: "live",
				Note:   "free SEC Number product only; not name search / GIS / AFS",
				Raw:    data,
			}
			if prov.Source != "" {
				view.Source = prov.Source
			}
			fillCompanyShowFromJSON(&view, data)
			return emitCompanyShow(cmd, flags, view)
		},
	}
	cmd.Flags().StringVar(&flagSecNo, "sec-no", "", "SEC registration number (must already know it — no name search)")
	cmd.Flags().BoolVar(&flagRefresh, "refresh", false, "Call the live free SEC Number API (requires PH_SEC_TOKEN; burns free quota)")
	cmd.Flags().StringVar(&flagDB, "db", "", "Local SQLite path (default: data dir data.db)")
	return cmd
}

func loadCompanyShowFromStore(cmd *cobra.Command, db *store.Store, secNo string) (companyShowView, bool, error) {
	// id is usually sec_no after free-product sync; also match typed sec_no column.
	if raw, err := db.Get("companies", secNo); err == nil && len(raw) > 0 {
		view := companyShowView{SecNo: secNo, Source: "local", Raw: raw}
		fillCompanyShowFromJSON(&view, raw)
		return view, true, nil
	}
	rows, err := db.DB().QueryContext(cmd.Context(),
		`SELECT data FROM companies WHERE sec_no = ? OR id = ? LIMIT 1`, secNo, secNo)
	if err != nil {
		return companyShowView{}, false, nil
	}
	defer rows.Close()
	if !rows.Next() {
		return companyShowView{}, false, rows.Err()
	}
	var data []byte
	if err := rows.Scan(&data); err != nil {
		return companyShowView{}, false, err
	}
	view := companyShowView{SecNo: secNo, Source: "local", Raw: data}
	fillCompanyShowFromJSON(&view, data)
	return view, true, nil
}

func fillCompanyShowFromJSON(view *companyShowView, data json.RawMessage) {
	var arr []map[string]any
	if json.Unmarshal(data, &arr) == nil && len(arr) > 0 {
		applyCompanyMap(view, arr[0])
		return
	}
	var obj map[string]any
	if json.Unmarshal(data, &obj) == nil {
		if nested, ok := obj["data"].([]any); ok && len(nested) > 0 {
			if m, ok := nested[0].(map[string]any); ok {
				applyCompanyMap(view, m)
				return
			}
		}
		applyCompanyMap(view, obj)
	}
}

func applyCompanyMap(view *companyShowView, m map[string]any) {
	if v, ok := m["sec_no"].(string); ok && v != "" {
		view.SecNo = v
	}
	if v, ok := m["company_name"].(string); ok {
		view.CompanyName = v
	}
	if v, ok := m["date_approved"].(string); ok {
		view.DateApproved = v
	}
	if v, ok := m["licenses"].(string); ok {
		view.Licenses = v
	}
	if v, ok := m["status"].(string); ok {
		view.Status = v
	}
}

func emitCompanyShow(cmd *cobra.Command, flags *rootFlags, view companyShowView) error {
	if flags.asJSON || flags.agent || (!isTerminal(cmd.OutOrStdout()) && !flags.csv && !flags.quiet && !flags.plain) {
		return printJSONFiltered(cmd.OutOrStdout(), view, flags)
	}
	fmt.Fprintf(cmd.OutOrStdout(), "sec_no:\t%s\n", view.SecNo)
	if view.CompanyName != "" {
		fmt.Fprintf(cmd.OutOrStdout(), "company_name:\t%s\n", view.CompanyName)
	}
	if view.DateApproved != "" {
		fmt.Fprintf(cmd.OutOrStdout(), "date_approved:\t%s\n", view.DateApproved)
	}
	if view.Licenses != "" {
		fmt.Fprintf(cmd.OutOrStdout(), "licenses:\t%s\n", view.Licenses)
	}
	if view.Status != "" {
		fmt.Fprintf(cmd.OutOrStdout(), "status:\t%s\n", view.Status)
	}
	fmt.Fprintf(cmd.OutOrStdout(), "source:\t%s\n", view.Source)
	if view.Note != "" {
		fmt.Fprintf(cmd.OutOrStdout(), "note:\t%s\n", view.Note)
	}
	return nil
}
