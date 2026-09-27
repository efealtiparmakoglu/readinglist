package rl

import (
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"testing"
	"time"
)

var day0 = time.Date(2026, 9, 27, 0, 0, 0, 0, time.UTC)

func mustReview(t *testing.T, s State, grade int, today time.Time) State {
	t.Helper()
	next, err := s.Review(grade, today)
	if err != nil {
		t.Fatalf("review: %v", err)
	}
	return next
}

func TestNewStateIsDueToday(t *testing.T) {
	state := NewState(day0)
	if state.Ease != 2.5 || state.Interval != 0 || state.Repetitions != 0 {
		t.Errorf("new state = %+v", state)
	}
	if !state.Due(day0) {
		t.Error("a new item should be due today")
	}
	if state.Stage() != "new" {
		t.Errorf("stage = %q", state.Stage())
	}
}

func TestFirstTwoIntervalsFollowSM2(t *testing.T) {
	first := mustReview(t, NewState(day0), 4, day0)
	if first.Interval != 1 {
		t.Errorf("first interval = %d, want 1", first.Interval)
	}
	second := mustReview(t, first, 4, day0)
	if second.Interval != 6 {
		t.Errorf("second interval = %d, want 6", second.Interval)
	}
	third := mustReview(t, second, 4, day0)
	// 6 * ease(2.5 unchanged at grade 4) = 15
	if third.Interval != 15 {
		t.Errorf("third interval = %d, want 15", third.Interval)
	}
}

func TestForgettingResetsTheChain(t *testing.T) {
	state := State{Ease: 2.5, Interval: 15, Repetitions: 3}
	lapsed := mustReview(t, state, 2, day0)
	if lapsed.Repetitions != 0 {
		t.Errorf("repetitions = %d, want 0", lapsed.Repetitions)
	}
	if lapsed.Interval != 1 {
		t.Errorf("interval = %d, want 1", lapsed.Interval)
	}
	if lapsed.DueAt != day0.AddDate(0, 0, 1).Format(DateFormat) {
		t.Errorf("due = %s", lapsed.DueAt)
	}
}

func TestEaseFloor(t *testing.T) {
	state := NewState(day0)
	for i := 0; i < 10; i++ {
		state = mustReview(t, state, 5, day0)
	}
	if state.Ease < MinEase {
		t.Errorf("ease = %v, must not drop below %v", state.Ease, MinEase)
	}
}

func TestEaseChangesWithGrade(t *testing.T) {
	easy := mustReview(t, NewState(day0), 5, day0)
	hard := mustReview(t, NewState(day0), 3, day0)
	if easy.Ease <= 2.5 {
		t.Errorf("grade 5 should raise ease, got %v", easy.Ease)
	}
	if hard.Ease >= 2.5 {
		t.Errorf("grade 3 should lower ease, got %v", hard.Ease)
	}
	if got := mustReview(t, NewState(day0), 0, day0).Ease; got >= 2.5 {
		t.Errorf("grade 0 should lower ease most, got %v", got)
	}
}

func TestIntervalIsCapped(t *testing.T) {
	state := NewState(day0)
	for i := 0; i < 30; i++ {
		state = mustReview(t, state, 5, day0)
	}
	if state.Interval > MaxInterval {
		t.Errorf("interval = %d, cap is %d", state.Interval, MaxInterval)
	}
}

func TestGradeValidation(t *testing.T) {
	for _, grade := range []int{-1, 6, 100} {
		if _, err := NewState(day0).Review(grade, day0); err == nil {
			t.Errorf("grade %d should be rejected", grade)
		}
	}
	for grade := 0; grade <= 5; grade++ {
		if _, err := NewState(day0).Review(grade, day0); err != nil {
			t.Errorf("grade %d should be accepted: %v", grade, err)
		}
	}
}

func TestDueAndOverdue(t *testing.T) {
	state := mustReview(t, NewState(day0), 4, day0) // due tomorrow
	today := day0
	later := day0.AddDate(0, 0, 1)
	if state.Due(today) {
		t.Error("not due today")
	}
	if !state.Due(later) {
		t.Error("due the next day")
	}
	if got := state.DaysUntilDue(today); got != 1 {
		t.Errorf("days until due = %d, want 1", got)
	}
	if got := state.DaysUntilDue(later); got != 0 {
		t.Errorf("days until due on the day = %d, want 0", got)
	}
}

func TestUnreadableDueDateCountsAsDue(t *testing.T) {
	state := State{Ease: 2.5, DueAt: "not-a-date"}
	if !state.Due(day0) {
		t.Error("a broken date should be treated as due now")
	}
	if state.DaysUntilDue(day0) != 0 {
		t.Error("a broken date should report zero days")
	}
}

func TestStages(t *testing.T) {
	cases := map[string]State{
		"new":      NewState(day0),
		"learning": {Ease: 2.5, Interval: 1, Repetitions: 1},
		"young":    {Ease: 2.5, Interval: 15, Repetitions: 3},
		"mature":   {Ease: 2.5, Interval: 40, Repetitions: 5},
	}
	for want, state := range cases {
		if got := state.Stage(); got != want {
			t.Errorf("stage for %+v = %q, want %q", state, got, want)
		}
	}
}

func TestAddAndFind(t *testing.T) {
	var list List
	item, err := list.Add("A Book", "An Author", []string{"Fiction, Sci-Fi", "fiction"}, day0)
	if err != nil {
		t.Fatal(err)
	}
	if item.ID != 1 {
		t.Errorf("first id = %d", item.ID)
	}
	if len(item.Tags) != 2 || item.Tags[0] != "fiction" {
		t.Errorf("tags = %v (should be deduped, lowercased, sorted)", item.Tags)
	}
	found, err := list.Find(1)
	if err != nil || found.Title != "A Book" {
		t.Errorf("find = %+v, %v", found, err)
	}
	if _, err := list.Find(99); err == nil {
		t.Error("expected an error for a missing id")
	}
}

func TestAddRequiresTitle(t *testing.T) {
	var list List
	if _, err := list.Add("   ", "", nil, day0); err == nil {
		t.Error("expected an error for a blank title")
	}
}

func TestReviewMarksCompletedOnGoodGrades(t *testing.T) {
	var list List
	if _, err := list.Add("Book", "", nil, day0); err != nil {
		t.Fatal(err)
	}
	item, err := list.Review(1, 4, day0)
	if err != nil {
		t.Fatal(err)
	}
	if !item.IsDone() {
		t.Error("grade 4 should mark the item done")
	}
	if item.Completed != day0.Format(DateFormat) {
		t.Errorf("completed = %q", item.Completed)
	}
}

func TestDueListingOrder(t *testing.T) {
	var list List
	if _, err := list.Add("Later", "", nil, day0); err != nil {
		t.Fatal(err)
	}
	if _, err := list.Add("Overdue", "", nil, day0); err != nil {
		t.Fatal(err)
	}
	// push item 1 into the future, leave item 2 due today
	if _, err := list.Review(1, 5, day0); err != nil {
		t.Fatal(err)
	}
	due := list.Due(day0)
	if len(due) != 1 || due[0].Title != "Overdue" {
		t.Errorf("due = %+v", due)
	}
}

func TestFilterAndTagCounts(t *testing.T) {
	var list List
	list.Add("A", "", []string{"go"}, day0)
	list.Add("B", "", []string{"go", "rust"}, day0)
	list.Add("C", "", []string{"rust"}, day0)

	if got := len(list.Filter("go")); got != 2 {
		t.Errorf("filter go = %d, want 2", got)
	}
	if got := len(list.Filter("")); got != 3 {
		t.Errorf("empty filter = %d, want 3", got)
	}
	tags := list.Tags()
	if len(tags) != 2 || tags[0].Tag != "go" || tags[0].Count != 2 {
		t.Errorf("tags = %+v (ties should sort alphabetically)", tags)
	}
}

func TestSummary(t *testing.T) {
	var list List
	list.Add("Fresh", "", nil, day0) // new, due today
	list.Add("Old", "", nil, day0)
	list.Review(2, 4, day0) // done, due tomorrow

	summary := list.Summarize(day0)
	if summary.Total != 2 {
		t.Errorf("total = %d", summary.Total)
	}
	if summary.New != 1 {
		t.Errorf("new = %d", summary.New)
	}
	if summary.Completed != 1 {
		t.Errorf("completed = %d", summary.Completed)
	}
	if summary.DueToday != 1 {
		t.Errorf("due today = %d", summary.DueToday)
	}
	if len(summary.DueItems) != 1 || summary.DueItems[0].Title != "Fresh" {
		t.Errorf("due items = %+v", summary.DueItems)
	}
}

func TestStreakCountsConsecutiveReviewDays(t *testing.T) {
	var list List
	list.Add("A", "", nil, day0)
	list.Add("B", "", nil, day0)
	// reviews on day0 and day0-1
	if _, err := list.Review(1, 4, day0); err != nil {
		t.Fatal(err)
	}
	if _, err := list.Review(2, 4, day0.AddDate(0, 0, -1)); err != nil {
		t.Fatal(err)
	}
	if got := list.Streak(day0); got != 2 {
		t.Errorf("streak = %d, want 2", got)
	}

	// a gap breaks the streak
	var gap List
	gap.Add("A", "", nil, day0)
	gap.Review(1, 4, day0.AddDate(0, 0, -3))
	if got := gap.Streak(day0); got != 0 {
		t.Errorf("streak after a gap = %d, want 0", got)
	}
}

func TestStreakWithNothingTodayKeepsYesterdays(t *testing.T) {
	var list List
	list.Add("A", "", nil, day0)
	list.Review(1, 4, day0.AddDate(0, 0, -1))
	if got := list.Streak(day0); got != 1 {
		t.Errorf("streak = %d, want 1", got)
	}
}

func TestSaveAndLoadRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nested", "readinglist.json")
	var list List
	list.Add("Book", "Author", []string{"tag"}, day0)
	list.Review(1, 4, day0)
	if err := Save(path, list); err != nil {
		t.Fatal(err)
	}
	loaded, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(loaded.Items) != 1 || loaded.Items[0].Title != "Book" {
		t.Errorf("loaded = %+v", loaded)
	}
	if loaded.Items[0].State.Repetitions != 1 {
		t.Errorf("state lost: %+v", loaded.Items[0].State)
	}
	if loaded.Next != 2 {
		t.Errorf("next id = %d, want 2", loaded.Next)
	}
}

func TestLoadMissingFileIsEmpty(t *testing.T) {
	list, err := Load(filepath.Join(t.TempDir(), "absent.json"))
	if err != nil {
		t.Fatal(err)
	}
	if len(list.Items) != 0 {
		t.Errorf("expected an empty list, got %+v", list)
	}
}

func TestLoadEmptyFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "empty.json")
	os.WriteFile(path, nil, 0o644)
	list, err := Load(path)
	if err != nil || len(list.Items) != 0 {
		t.Errorf("list = %+v, err = %v", list, err)
	}
}

func TestLoadCorruptFileFails(t *testing.T) {
	path := filepath.Join(t.TempDir(), "corrupt.json")
	os.WriteFile(path, []byte("{not json"), 0o644)
	if _, err := Load(path); err == nil {
		t.Error("expected an error for a corrupt file")
	}
}

func TestLoadHealsMissingNextID(t *testing.T) {
	path := filepath.Join(t.TempDir(), "old.json")
	payload := `{"items":[{"id":7,"title":"Old","state":{"ease":2.5,"interval_days":1,` +
		`"repetitions":1,"due_at":"2026-09-28"}}]}`
	os.WriteFile(path, []byte(payload), 0o644)
	list, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if list.Next != 8 {
		t.Errorf("next id = %d, want 8", list.Next)
	}
}

func TestEaseRoundingIsStable(t *testing.T) {
	state := mustReview(t, NewState(day0), 3, day0)
	if math.Round(state.Ease*100)/100 != state.Ease {
		t.Errorf("ease %v is not rounded to two decimals", state.Ease)
	}
}

func TestJSONShape(t *testing.T) {
	var list List
	list.Add("Book", "Author", []string{"tag"}, day0)
	data, err := json.Marshal(list)
	if err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{`"title"`, `"tags"`, `"ease"`, `"due_at"`, `"next_id"`} {
		if !contains(string(data), key) {
			t.Errorf("JSON is missing %s: %s", key, data)
		}
	}
}

func contains(haystack, needle string) bool {
	return len(haystack) >= len(needle) &&
		(haystack == needle || indexOf(haystack, needle) >= 0)
}

func indexOf(haystack, needle string) int {
	for i := 0; i+len(needle) <= len(haystack); i++ {
		if haystack[i:i+len(needle)] == needle {
			return i
		}
	}
	return -1
}
