// Command readinglist — a local reading list scheduled with SM-2 spaced
// repetition, with tags and a daily summary.
package main

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/efealtiparmakoglu/readinglist/rl"
)

const version = "1.0.0"
const defaultPath = "rl.json"

const usage = `readinglist %s — local reading list with SM-2 spaced repetition

usage:
  readinglist add <title> [--author NAME] [--tag a,b] [--file PATH]
  readinglist due [--file PATH] [--json]
  readinglist review <id> <grade 0-5> [--file PATH]
  readinglist list [--tag TAG] [--all] [--file PATH]
  readinglist tags [--file PATH]
  readinglist summary [--file PATH] [--json]
  readinglist export --csv <file> [--file PATH]
  readinglist -h | --help

grades: 0-2 forgotten (back tomorrow), 3 hard, 4 good, 5 easy
exit codes: 0 ok, 1 runtime error, 2 usage error
`

func main() {
	os.Exit(run(os.Args[1:]))
}

func today() time.Time {
	now := time.Now()
	return time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
}

func run(args []string) int {
	if len(args) == 0 || args[0] == "-h" || args[0] == "--help" {
		fmt.Fprintf(os.Stderr, usage, version)
		return 2
	}
	command := args[0]
	rest := args[1:]

	path := defaultPath
	tag := ""
	author := ""
	csvPath := ""
	asJSON := false
	all := false
	var positional []string

	for i := 0; i < len(rest); i++ {
		arg := rest[i]
		value := func(name string) string {
			if i+1 < len(rest) {
				i++
				return rest[i]
			}
			return ""
		}
		switch arg {
		case "--file":
			path = value("--file")
		case "--tag":
			tag = value("--tag")
		case "--author":
			author = value("--author")
		case "--csv":
			csvPath = value("--csv")
		case "--json":
			asJSON = true
		case "--all":
			all = true
		default:
			positional = append(positional, arg)
		}
	}

	list, err := rl.Load(path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "readinglist: %v\n", err)
		return 1
	}
	day := today()

	switch command {
	case "add":
		if len(positional) == 0 {
			fmt.Fprintln(os.Stderr, "readinglist: add needs a title")
			return 2
		}
		item, err := list.Add(strings.Join(positional, " "), author, []string{tag}, day)
		if err != nil {
			fmt.Fprintf(os.Stderr, "readinglist: %v\n", err)
			return 1
		}
		if err := rl.Save(path, list); err != nil {
			fmt.Fprintf(os.Stderr, "readinglist: %v\n", err)
			return 1
		}
		fmt.Printf("#%d %s (due %s)\n", item.ID, item.Title, item.State.DueAt)
		return 0

	case "due":
		due := list.Due(day)
		if asJSON {
			return printJSON(due)
		}
		if len(due) == 0 {
			fmt.Println("nothing due today")
			return 0
		}
		fmt.Printf("%d due on %s\n", len(due), day.Format(rl.DateFormat))
		for _, item := range due {
			overdue := item.State.DaysUntilDue(day)
			suffix := ""
			if overdue < 0 {
				suffix = fmt.Sprintf(" (%d days overdue)", -overdue)
			}
			fmt.Printf(" #%d %-40s ease %.2f%s\n", item.ID, truncate(item.Title, 40),
				item.State.Ease, suffix)
		}
		return 0

	case "review":
		if len(positional) < 2 {
			fmt.Fprintln(os.Stderr, "readinglist: review needs an id and a grade")
			return 2
		}
		id, err := strconv.Atoi(positional[0])
		if err != nil {
			fmt.Fprintf(os.Stderr, "readinglist: id must be a number: %v\n", err)
			return 2
		}
		grade, err := strconv.Atoi(positional[1])
		if err != nil {
			fmt.Fprintf(os.Stderr, "readinglist: grade must be a number: %v\n", err)
			return 2
		}
		item, err := list.Review(id, grade, day)
		if err != nil {
			fmt.Fprintf(os.Stderr, "readinglist: %v\n", err)
			return 1
		}
		if err := rl.Save(path, list); err != nil {
			fmt.Fprintf(os.Stderr, "readinglist: %v\n", err)
			return 1
		}
		fmt.Printf("#%d %s -> grade %d, next review %s (interval %d days)\n",
			item.ID, item.Title, item.State.LastGrade, item.State.DueAt,
			item.State.Interval)
		return 0

	case "list":
		items := list.Filter(tag)
		if !all {
			var open []rl.Item
			for _, item := range items {
				if !item.IsDone() {
					open = append(open, item)
				}
			}
			items = open
		}
		if asJSON {
			return printJSON(items)
		}
		if len(items) == 0 {
			fmt.Println("(nothing yet)")
			return 0
		}
		for _, item := range items {
			state := "open"
			if item.IsDone() {
				state = "done"
			}
			line := fmt.Sprintf("#%d %-40s %-6s due %s", item.ID, truncate(item.Title, 40),
				state, item.State.DueAt)
			if len(item.Tags) > 0 {
				line += "  [" + strings.Join(item.Tags, ",") + "]"
			}
			fmt.Println(line)
		}
		return 0

	case "tags":
		return printJSON(list.Tags())
	case "summary":
		summary := list.Summarize(day)
		if asJSON {
			return printJSON(summary)
		}
		fmt.Printf("reading list for %s\n", summary.Date)
		fmt.Printf("  total:     %d (%d done)\n", summary.Total, summary.Completed)
		fmt.Printf("  due today: %d (%d overdue)\n", summary.DueToday, summary.Overdue)
		fmt.Printf("  stages:    %d new, %d learning, %d young, %d mature\n",
			summary.New, summary.Learning, summary.Young, summary.Mature)
		fmt.Printf("  streak:    %d day(s)\n", list.Streak(day))
		if len(summary.ByTag) > 0 {
			fmt.Println("  tags:")
			for _, tag := range summary.ByTag {
				fmt.Printf("    %-12s %d\n", tag.Tag, tag.Count)
			}
		}
		return 0

	case "export":
		if csvPath == "" {
			fmt.Fprintln(os.Stderr, "readinglist: export needs --csv")
			return 2
		}
		if err := writeCSV(csvPath, list.Items); err != nil {
			fmt.Fprintf(os.Stderr, "readinglist: %v\n", err)
			return 1
		}
		fmt.Printf("wrote %d row(s) to %s\n", len(list.Items), csvPath)
		return 0

	default:
		fmt.Fprintf(os.Stderr, "readinglist: unknown command '%s'\n", command)
		fmt.Fprintf(os.Stderr, usage, version)
		return 2
	}
}

func writeCSV(path string, items []rl.Item) error {
	handle, err := os.Create(path)
	if err != nil {
		return err
	}
	defer handle.Close()
	writer := csv.NewWriter(handle)
	defer writer.Flush()
	if err := writer.Write([]string{"id", "title", "author", "tags", "added",
		"due", "interval", "ease", "repetitions", "completed"}); err != nil {
		return err
	}
	for _, item := range items {
		record := []string{
			strconv.Itoa(item.ID), item.Title, item.Author,
			strings.Join(item.Tags, ";"), item.AddedAt, item.State.DueAt,
			strconv.Itoa(item.State.Interval),
			strconv.FormatFloat(item.State.Ease, 'f', 2, 64),
			strconv.Itoa(item.State.Repetitions), item.Completed,
		}
		if err := writer.Write(record); err != nil {
			return err
		}
	}
	writer.Flush()
	return writer.Error()
}

func printJSON(value any) int {
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		fmt.Fprintf(os.Stderr, "readinglist: %v\n", err)
		return 1
	}
	fmt.Println(string(data))
	return 0
}

func truncate(text string, width int) string {
	if len(text) <= width {
		return text
	}
	if width <= 3 {
		return text[:width]
	}
	return text[:width-3] + "..."
}
