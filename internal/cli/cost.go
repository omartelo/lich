package cli

import (
	"encoding/csv"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/omartelo/lich/internal/store"
)

func (c *client) cost(args []string) error {
	flags := newFlagSet("cost")
	project := flags.String("project", "", "only this project, by name")
	provider := flags.String("provider", "", "only sessions running this provider")
	since := flags.String("since", "", "only sessions active within this window, e.g. 7d or 12h")
	asJSON := flags.Bool("json", false, "print the whole report as JSON")
	asCSV := flags.Bool("csv", false, "print the per-project rows as CSV")
	if err := c.parse(flags, args); err != nil {
		return err
	}
	// One format or the other: asked for both, there is no answer to print that
	// is not half of each.
	if flags.NArg() != 0 || (*asJSON && *asCSV) {
		return usageError("cost")
	}
	from, err := windowStart(*since)
	if err != nil {
		return err
	}

	var report store.CostReport
	if err := c.call("store.CostTotals", []any{*project, *provider, from}, shortCall, &report); err != nil {
		return err
	}
	report.Projects = asList(report.Projects)
	switch {
	case *asJSON:
		return c.emit(report)
	case *asCSV:
		return c.emitCSV(report)
	}
	return c.printCost(report)
}

// windowStart turns a --since value into the unix second the report starts at,
// or 0 for no window at all. Days are spelled out here because Go's duration
// parser stops at hours and "what did this week cost" is the question the
// command is asked.
func windowStart(value string) (int64, error) {
	if value == "" {
		return 0, nil
	}
	span, err := parseSpan(value)
	if err != nil || span <= 0 {
		return 0, fmt.Errorf("--since takes a window like 7d, 24h or 90m, not %q", value)
	}
	return time.Now().Add(-span).Unix(), nil
}

func parseSpan(value string) (time.Duration, error) {
	if days, ok := strings.CutSuffix(value, "d"); ok {
		n, err := strconv.Atoi(days)
		if err != nil {
			return 0, err
		}
		return time.Duration(n) * 24 * time.Hour, nil
	}
	return time.ParseDuration(value)
}

// printCost writes the report as a table and then the line the total is never
// printed without: what it leaves out. A sum over sessions lich could not price
// is a lower bound, and one that says so is the difference between a number
// worth acting on and a number that quietly reads as the whole bill.
func (c *client) printCost(report store.CostReport) error {
	if report.Sessions == 0 {
		fmt.Fprintln(c.stdout, "No sessions matched.")
		return nil
	}
	fmt.Fprintln(c.stdout, "project\tsessions\tunpriced\tsource\tcost")
	for _, row := range report.Projects {
		fmt.Fprintf(c.stdout, "%s\t%d\t%d\t%s\t%s\n",
			row.Project, row.Sessions, row.Unpriced, source(row.Source), money(row.CostUSD))
	}
	fmt.Fprintf(c.stdout, "total\t%d\t%d\t%s\t%s\n",
		report.Sessions, report.Unpriced, source(report.Source), money(report.CostUSD))
	if line := arithmetic(report); line != "" {
		fmt.Fprintln(c.stdout, line)
	}
	fmt.Fprintln(c.stdout, exclusion(report))
	return nil
}

// source is a row's rung as a table cell. A row with nothing to attribute — no
// money counted in it at all — reads as the absence it is rather than as a gap
// in the column.
func source(rung string) string {
	if rung == "" {
		return "—"
	}
	return rung
}

// arithmetic names the split when a total spans both rungs. A dollar lich
// derived from token counts and a dollar the provider reported are the same
// figure in the money column, and the sum is the one place that difference
// disappears — the source column has already said which is which per row, so a
// total on a single rung needs no line at all.
func arithmetic(report store.CostReport) string {
	if report.Priced == 0 || report.Reported == 0 {
		return ""
	}
	return fmt.Sprintf(
		"%d priced by lich, %d reported by their provider.", report.Priced, report.Reported,
	)
}

// exclusion words what the total does not cover. Off readout gets its own
// sentence: it is the reason every session is unpriced, and without it a
// machine that has simply never been asked to count reads as one that spent
// nothing.
func exclusion(report store.CostReport) string {
	line := "Complete: every session in this total is priced."
	if report.Unpriced > 0 {
		line = fmt.Sprintf(
			"Lower bound: %d unpriced of %d sessions — their spend is not in this total.",
			report.Unpriced, report.Sessions,
		)
	}
	if !report.Readout {
		line += "\nThe cost readout is off, so nothing is being priced — turn on" +
			` Settings › "Session readout in the footer" › "And cost".`
	}
	return line
}

// money is a figure as a command line shows it. Two places because that is what
// a bill reads in; --json and --csv carry the exact number, which is what a
// script adding these up needs.
func money(usd float64) string {
	return fmt.Sprintf("$%.2f", usd)
}

// emitCSV writes the per-project rows and no total: a total is the sum of the
// columns, and the unpriced and source columns travel beside the money, so what
// the number leaves out and whose arithmetic it is survive the export rather
// than staying behind in lich.
func (c *client) emitCSV(report store.CostReport) error {
	rows := [][]string{{"project", "sessions", "unpriced", "source", "cost_usd"}}
	for _, row := range report.Projects {
		rows = append(rows, []string{
			row.Project,
			strconv.Itoa(row.Sessions),
			strconv.Itoa(row.Unpriced),
			// Empty rather than the table's dash: a script reads a blank cell as
			// "no rung", and the dash is a thing to print, not a value.
			row.Source,
			strconv.FormatFloat(row.CostUSD, 'f', 6, 64),
		})
	}
	w := csv.NewWriter(c.stdout)
	if err := w.WriteAll(rows); err != nil {
		return fmt.Errorf("write the rows: %w", err)
	}
	return nil
}
