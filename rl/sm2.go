// Package rl keeps a local reading list scheduled with SM-2
// spaced repetition, plus tags and a daily summary. Standard library only.
package rl

import (
	"fmt"
	"math"
	"sort"
	"strings"
	"time"
)

const (
	MinEase     = 1.3
	FirstSecond = 1
	SecondDay   = 6
	MaxInterval = 365 * 5
)

// State is the SM-2 scheduling state of one item.
type State struct {
	Ease        float64 `json:"ease"`
	Interval    int     `json:"interval_days"`
	Repetitions int     `json:"repetitions"`
	DueAt       string  `json:"due_at"`
	LastGrade   int     `json:"last_grade,omitempty"`
	LastReview  string  `json:"last_review,omitempty"`
}

// NewState is the state of something never studied: due today.
func NewState(today time.Time) State {
	return State{
		Ease:     2.5,
		Interval: 0,
		DueAt:    today.Format(DateFormat),
	}
}

// Grade validates a 0..5 recall grade.
func Grade(g int) (int, error) {
	if g < 0 || g > 5 {
		return 0, fmt.Errorf("grade must be 0..5, got %d", g)
	}
	return g, nil
}

// Review applies the SM-2 transition and returns the updated state.
func (s State) Review(grade int, today time.Time) (State, error) {
	grade, err := Grade(grade)
	if err != nil {
		return s, err
	}
	next := s
	next.LastGrade = grade
	next.LastReview = today.Format(DateFormat)

	// ease factor update, floored at 1.3 (SM-2's own rule)
	delta := 0.1 - float64(5-grade)*(0.08+float64(5-grade)*0.02)
	ease := s.Ease + delta
	if ease < MinEase {
		ease = MinEase
	}
	next.Ease = math.Round(ease*100) / 100

	if grade < 3 {
		// forgotten: back to the start, review again tomorrow
		next.Repetitions = 0
		next.Interval = 1
		next.DueAt = today.AddDate(0, 0, 1).Format(DateFormat)
		return next, nil
	}

	next.Repetitions = s.Repetitions + 1
	switch next.Repetitions {
	case 1:
		next.Interval = FirstSecond
	case 2:
		next.Interval = SecondDay
	default:
		next.Interval = int(math.Round(float64(s.Interval) * next.Ease))
	}
	if next.Interval > MaxInterval {
		next.Interval = MaxInterval
	}
	next.DueAt = today.AddDate(0, 0, next.Interval).Format(DateFormat)
	return next, nil
}

// Due reports whether the item is due on the given day.
func (s State) Due(today time.Time) bool {
	due, err := time.Parse(DateFormat, s.DueAt)
	if err != nil {
		return true // an unreadable date is treated as due now
	}
	return !due.After(today)
}

// DaysUntilDue is negative for overdue items.
func (s State) DaysUntilDue(today time.Time) int {
	due, err := time.Parse(DateFormat, s.DueAt)
	if err != nil {
		return 0
	}
	todayMidnight := time.Date(today.Year(), today.Month(), today.Day(),
		0, 0, 0, 0, today.Location())
	return int(due.Sub(todayMidnight).Hours() / 24)
}

// Stage classifies an item for the summary.
func (s State) Stage() string {
	switch {
	case s.Repetitions == 0 && s.Interval == 0:
		return "new"
	case s.Repetitions < 3:
		return "learning"
	case s.Interval < 21:
		return "young"
	default:
		return "mature"
	}
}

// DateFormat is the calendar date format used throughout.
const DateFormat = "2006-01-02"

// Item is one book in the list.
type Item struct {
	ID        int      `json:"id"`
	Title     string   `json:"title"`
	Author    string   `json:"author,omitempty"`
	Tags      []string `json:"tags,omitempty"`
	Notes     string   `json:"notes,omitempty"`
	AddedAt   string   `json:"added_at"`
	Completed string   `json:"completed_at,omitempty"`
	State     State    `json:"state"`
}

func (i Item) HasTag(tag string) bool {
	for _, t := range i.Tags {
		if strings.EqualFold(t, tag) {
			return true
		}
	}
	return false
}

func (i Item) IsDone() bool { return i.Completed != "" }

// List is the whole collection.
type List struct {
	Items []Item `json:"items"`
	Next  int    `json:"next_id"`
}

// Add appends an item and returns it.
func (l *List) Add(title, author string, tags []string, today time.Time) (Item, error) {
	if strings.TrimSpace(title) == "" {
		return Item{}, fmt.Errorf("a title is required")
	}
	if l.Next == 0 {
		l.Next = 1
	}
	item := Item{
		ID:      l.Next,
		Title:   strings.TrimSpace(title),
		Author:  strings.TrimSpace(author),
		Tags:    normalizeTags(tags),
		AddedAt: today.Format(DateFormat),
		State:   NewState(today),
	}
	l.Next++
	l.Items = append(l.Items, item)
	return item, nil
}

func normalizeTags(tags []string) []string {
	seen := map[string]bool{}
	var out []string
	for _, tag := range tags {
		for _, part := range strings.Split(tag, ",") {
			clean := strings.ToLower(strings.TrimSpace(part))
			if clean == "" || seen[clean] {
				continue
			}
			seen[clean] = true
			out = append(out, clean)
		}
	}
	sort.Strings(out)
	return out
}

// Find returns the item with the given id.
func (l *List) Find(id int) (*Item, error) {
	for i := range l.Items {
		if l.Items[i].ID == id {
			return &l.Items[i], nil
		}
	}
	return nil, fmt.Errorf("no item with id %d", id)
}

// Review grades an item and schedules its next appearance.
func (l *List) Review(id, grade int, today time.Time) (Item, error) {
	item, err := l.Find(id)
	if err != nil {
		return Item{}, err
	}
	updated, err := item.State.Review(grade, today)
	if err != nil {
		return Item{}, err
	}
	item.State = updated
	if grade >= 4 {
		item.Completed = today.Format(DateFormat)
	}
	return *item, nil
}

// Due returns the items due on the given day, newest deadline first.
func (l *List) Due(today time.Time) []Item {
	var due []Item
	for _, item := range l.Items {
		if item.State.Due(today) {
			due = append(due, item)
		}
	}
	sort.SliceStable(due, func(i, j int) bool {
		return due[i].State.DueAt < due[j].State.DueAt
	})
	return due
}

// Filter returns items carrying the tag ("" means all).
func (l *List) Filter(tag string) []Item {
	if tag == "" {
		return append([]Item{}, l.Items...)
	}
	var out []Item
	for _, item := range l.Items {
		if item.HasTag(tag) {
			out = append(out, item)
		}
	}
	return out
}

// Tags returns every tag in use with its item count.
func (l *List) Tags() []TagCount {
	counts := map[string]int{}
	for _, item := range l.Items {
		for _, tag := range item.Tags {
			counts[tag]++
		}
	}
	out := make([]TagCount, 0, len(counts))
	for tag, count := range counts {
		out = append(out, TagCount{Tag: tag, Count: count})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Count != out[j].Count {
			return out[i].Count > out[j].Count
		}
		return out[i].Tag < out[j].Tag
	})
	return out
}

// TagCount is a tag with how many items carry it.
type TagCount struct {
	Tag   string `json:"tag"`
	Count int    `json:"count"`
}

// Summary is the daily digest.
type Summary struct {
	Date      string     `json:"date"`
	Total     int        `json:"total"`
	DueToday  int        `json:"due_today"`
	Overdue   int        `json:"overdue"`
	New       int        `json:"new"`
	Learning  int        `json:"learning"`
	Young     int        `json:"young"`
	Mature    int        `json:"mature"`
	Completed int        `json:"completed"`
	ByTag     []TagCount `json:"by_tag"`
	DueItems  []Item     `json:"due_items"`
}

// Summarize builds the daily summary.
func (l *List) Summarize(today time.Time) Summary {
	summary := Summary{Date: today.Format(DateFormat), Total: len(l.Items)}
	for _, item := range l.Items {
		switch item.State.Stage() {
		case "new":
			summary.New++
		case "learning":
			summary.Learning++
		case "young":
			summary.Young++
		case "mature":
			summary.Mature++
		}
		if item.IsDone() {
			summary.Completed++
		}
		if item.State.Due(today) {
			summary.DueToday++
			if item.State.DaysUntilDue(today) < 0 {
				summary.Overdue++
			}
		}
	}
	summary.ByTag = l.Tags()
	summary.DueItems = l.Due(today)
	return summary
}

// Streak counts consecutive days ending today with at least one review.
func (l *List) Streak(today time.Time) int {
	days := map[string]bool{}
	for _, item := range l.Items {
		if item.State.LastReview != "" {
			days[item.State.LastReview] = true
		}
		if item.Completed != "" {
			days[item.Completed] = true
		}
	}
	streak := 0
	for offset := 0; ; offset++ {
		day := today.AddDate(0, 0, -offset)
		if days[day.Format(DateFormat)] {
			streak++
			continue
		}
		if offset == 0 {
			// nothing done today yet: the streak is still yesterday's
			continue
		}
		break
	}
	return streak
}
