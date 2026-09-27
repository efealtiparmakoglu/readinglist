# readinglist

**A local reading list scheduled with SM-2 spaced repetition — add books, tag them, review with a 0–5 grade, and get a daily summary. Go standard library only, one JSON file.**

```
$ readinglist add "Structure and Interpretation" --author Abelson --tag cs
#1 Structure and Interpretation (due 2026-09-27)

$ readinglist review 1 4
#1 Structure and Interpretation -> grade 4, next review 2026-09-28 (interval 1 days)
$ readinglist review 1 5
#1 Structure and Interpretation -> grade 5, next review 2026-10-13 (interval 16 days)
$ readinglist review 1 5
#1 Structure and Interpretation -> grade 5, next review 2027-01-05 (interval 100 days)

$ readinglist summary
reading list for 2026-09-27
  total:     2 (1 done)
  due today: 0 (0 overdue)
  stages:    1 new, 0 learning, 0 young, 1 mature
  streak:    1 day(s)
  tags:
    cs           2
```

Books you meant to read are books you never come back to. readinglist
keeps the list, and keeps the schedule: SM-2 decides when each book
comes back, and the interval grows as long as you remember it.

## Why readinglist

- **Real SM-2, not a TODO list with dates.** Ease factor per item, the
  1-day / 6-day / interval×ease progression, the ease update formula
  with its 1.3 floor, and the 5-year interval cap. A grade of 0–2
  forgets the book and brings it back tomorrow.
- **The daily view is the point.** `due` lists what to read today,
  overdue items first with how late they are; `summary` shows the mix of
  new / learning / young / mature items and your current streak.
- **Tags that behave.** Case- and spacing-insensitive, deduplicated,
  sorted, with counts — `--tag cs,work` adds both.
- **Honest file handling.** Missing file = empty list, empty file =
  empty list, corrupt file = an error rather than silent data loss, and
  a file written by an older version with a missing `next_id` is healed
  on load.
- **Nothing leaves the machine.** One JSON file, no account, no sync, no
  network calls anywhere in the code.
- **Exportable.** `--csv` writes the full schedule — ease, interval,
  repetitions, due date — for whatever you want to do with it next.

## Usage

```sh
readinglist add "A Book" [--author NAME] [--tag a,b] [--file PATH]
readinglist due [--json] [--file PATH]
readinglist review <id> <0-5> [--file PATH]
readinglist list [--tag TAG] [--all] [--json]
readinglist tags
readinglist summary [--json]
readinglist export --csv <file>
```

Grades: 0–2 forgotten (back tomorrow), 3 hard, 4 good, 5 easy.

Exit codes: 0 ok, 1 runtime error (unknown id, unreadable file), 2 usage
error.

## Development

```sh
gofmt -l .
go vet ./...
go test -race ./...
```

The tests pin the SM-2 transitions against the algorithm's own numbers
(1, 6, 15…), the ease floor and cap, the stage boundaries, streak
accounting across gaps, and every persistence path including the
self-healing load.

## License

MIT — see [LICENSE](LICENSE).
