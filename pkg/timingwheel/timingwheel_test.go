package timingwheel

import (
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func newTW() *TimingWheel {
	return NewTimingWheel(10*time.Millisecond, 60)
}

func TestSlotRemove_Middle(t *testing.T) {
	t.Parallel()

	s := &slot{}
	t0 := &Timer{id: "a", slotIdx: 0}
	t1 := &Timer{id: "b", slotIdx: 1}
	t2 := &Timer{id: "c", slotIdx: 2}
	s.timers = []*Timer{t0, t1, t2}

	s.remove(1)

	if len(s.timers) != 2 {
		t.Fatalf("expected 2 timers, got %d", len(s.timers))
	}

	if s.timers[1].id != "c" {
		t.Errorf("expected 'c' at index 1, got %q", s.timers[1].id)
	}
	if s.timers[1].slotIdx != 1 {
		t.Errorf("expected slotIdx=1, got %d", s.timers[1].slotIdx)
	}
}

func TestSlotRemove_Last(t *testing.T) {
	t.Parallel()

	s := &slot{}
	t0 := &Timer{id: "a", slotIdx: 0}
	t1 := &Timer{id: "b", slotIdx: 1}
	s.timers = []*Timer{t0, t1}

	s.remove(1)

	if len(s.timers) != 1 {
		t.Fatalf("expected 1 timer, got %d", len(s.timers))
	}
	if s.timers[0].id != "a" {
		t.Errorf("expected 'a', got %q", s.timers[0].id)
	}
}

func TestSlotRemove_Single(t *testing.T) {
	t.Parallel()

	s := &slot{timers: []*Timer{{id: "only", slotIdx: 0}}}
	s.remove(0)
	if len(s.timers) != 0 {
		t.Fatalf("expected 0 timers after removing the only one")
	}
}

func TestAdd_BasicPresence(t *testing.T) {
	t.Parallel()

	tw := newTW()
	defer tw.Stop()

	tw.Add("t1", 200*time.Millisecond, false, func() {})

	active := tw.ListActive()
	if _, ok := active["t1"]; !ok {
		t.Error("timer 't1' should be listed as active after Add")
	}
}

func TestAdd_OverwriteExisting(t *testing.T) {
	t.Parallel()

	tw := newTW()
	defer tw.Stop()

	tw.Add("t1", 500*time.Millisecond, false, func() {})
	tw.Add("t1", 100*time.Millisecond, false, func() {})

	active := tw.ListActive()
	if len(active) != 1 {
		t.Errorf("expected 1 active timer after overwrite, got %d", len(active))
	}
}

func TestAdd_MultipleTimers(t *testing.T) {
	t.Parallel()

	tw := newTW()
	defer tw.Stop()

	ids := []string{"a", "b", "c", "d"}
	for _, id := range ids {
		tw.Add(id, 300*time.Millisecond, false, func() {})
	}

	active := tw.ListActive()
	for _, id := range ids {
		if _, ok := active[id]; !ok {
			t.Errorf("timer %q missing from active list", id)
		}
	}
}

func TestRemove_Existing(t *testing.T) {
	t.Parallel()

	tw := newTW()
	defer tw.Stop()

	tw.Add("t1", 500*time.Millisecond, false, func() {})
	tw.Remove("t1")

	active := tw.ListActive()
	if _, ok := active["t1"]; ok {
		t.Error("timer 't1' should not be active after Remove")
	}
}

func TestRemove_NonExistent(t *testing.T) {
	t.Parallel()

	tw := newTW()
	defer tw.Stop()

	tw.Remove("ghost")
}

func TestRemove_IndexConsistency(t *testing.T) {
	t.Parallel()

	tw := newTW()
	defer tw.Stop()

	tw.Add("a", 300*time.Millisecond, false, func() {})
	tw.Add("b", 300*time.Millisecond, false, func() {})
	tw.Add("c", 300*time.Millisecond, false, func() {})

	tw.Remove("b")

	active := tw.ListActive()
	if _, ok := active["b"]; ok {
		t.Error("'b' should be gone")
	}
	if _, ok := active["a"]; !ok {
		t.Error("'a' should still be present")
	}
	if _, ok := active["c"]; !ok {
		t.Error("'c' should still be present")
	}
}

func TestUpdateTTL_NonExistent(t *testing.T) {
	t.Parallel()

	tw := newTW()
	defer tw.Stop()

	if tw.UpdateTTL("ghost", 100*time.Millisecond) {
		t.Error("UpdateTTL on non-existent id should return false")
	}
}

func TestUpdateTTL_NewInterval(t *testing.T) {
	t.Parallel()

	tw := newTW()
	defer tw.Stop()

	tw.Add("t1", 500*time.Millisecond, false, func() {})
	ok := tw.UpdateTTL("t1", 200*time.Millisecond)
	if !ok {
		t.Fatal("UpdateTTL should return true for existing id")
	}

	active := tw.ListActive()
	if _, ok := active["t1"]; !ok {
		t.Error("timer should still be active after UpdateTTL with positive interval")
	}
}

func TestUpdateTTL_ZeroInterval_FiresImmediately(t *testing.T) {
	t.Parallel()

	tw := newTW()
	defer tw.Stop()

	fired := make(chan struct{}, 1)
	tw.Add("t1", 500*time.Millisecond, false, func() {
		fired <- struct{}{}
	})

	ok := tw.UpdateTTL("t1", 0)
	if !ok {
		t.Fatal("UpdateTTL should return true for existing id")
	}

	select {
	case <-fired:
	case <-time.After(500 * time.Millisecond):
		t.Error("task should have been dispatched immediately when interval <= 0")
	}

	active := tw.ListActive()
	if _, ok := active["t1"]; ok {
		t.Error("timer should not be active after zero-interval UpdateTTL")
	}
}

func TestUpdateTTL_NegativeInterval_FiresImmediately(t *testing.T) {
	t.Parallel()

	tw := newTW()
	defer tw.Stop()

	fired := make(chan struct{}, 1)
	tw.Add("t1", 500*time.Millisecond, false, func() {
		fired <- struct{}{}
	})

	tw.UpdateTTL("t1", -1)

	select {
	case <-fired:
	case <-time.After(500 * time.Millisecond):
		t.Error("task should fire immediately for negative interval")
	}
}

func TestTimer_FiresAfterInterval(t *testing.T) {
	t.Parallel()

	tw := newTW()
	defer tw.Stop()

	fired := make(chan struct{}, 1)
	tw.Add("t1", 50*time.Millisecond, false, func() {
		fired <- struct{}{}
	})

	select {
	case <-fired:
	case <-time.After(300 * time.Millisecond):
		t.Error("one-shot timer did not fire within expected window")
	}
}

func TestTimer_OneShot_RemovedAfterFire(t *testing.T) {
	t.Parallel()

	tw := newTW()
	defer tw.Stop()

	done := make(chan struct{}, 1)
	tw.Add("t1", 50*time.Millisecond, false, func() {
		done <- struct{}{}
	})

	<-done
	time.Sleep(50 * time.Millisecond)

	active := tw.ListActive()
	if _, ok := active["t1"]; ok {
		t.Error("one-shot timer should not be in active list after firing")
	}
}

func TestTimer_Repeat_FiresMultipleTimes(t *testing.T) {
	t.Parallel()

	tw := newTW()
	defer tw.Stop()

	var count atomic.Int64
	tw.Add("t1", 40*time.Millisecond, true, func() {
		count.Add(1)
	})

	time.Sleep(250 * time.Millisecond)
	tw.Remove("t1")

	c := count.Load()
	if c < 3 {
		t.Errorf("repeating timer expected at least 3 fires, got %d", c)
	}
}

func TestTimer_Repeat_StillActiveAfterFire(t *testing.T) {
	t.Parallel()

	tw := newTW()
	defer tw.Stop()

	fired := make(chan struct{}, 1)
	tw.Add("t1", 40*time.Millisecond, true, func() {
		select {
		case fired <- struct{}{}:
		default:
		}
	})

	<-fired
	time.Sleep(20 * time.Millisecond)

	active := tw.ListActive()
	if _, ok := active["t1"]; !ok {
		t.Error("repeating timer should remain active after firing")
	}
}

func TestListActive_RemainingTimeApproximate(t *testing.T) {
	t.Parallel()

	tw := newTW()
	defer tw.Stop()

	tw.Add("t1", 200*time.Millisecond, false, func() {})

	active := tw.ListActive()
	remaining, ok := active["t1"]
	if !ok {
		t.Fatal("t1 not found in active list")
	}

	if remaining <= 0 || remaining > 300*time.Millisecond {
		t.Errorf("unexpected remaining time: %v", remaining)
	}
}

func TestListActive_Empty(t *testing.T) {
	t.Parallel()

	tw := newTW()
	defer tw.Stop()

	active := tw.ListActive()
	if len(active) != 0 {
		t.Errorf("expected empty active list, got %d entries", len(active))
	}
}

func TestStop_NoMoreFires(t *testing.T) {
	t.Parallel()
	tw := newTW()

	var count atomic.Int64
	tw.Add("t1", 30*time.Millisecond, true, func() {
		count.Add(1)
	})

	time.Sleep(60 * time.Millisecond)
	tw.Stop()
	countAfterStop := count.Load()
	time.Sleep(100 * time.Millisecond)

	if count.Load() > countAfterStop+1 {
		t.Error("timer should not fire after Stop()")
	}
}

func TestConcurrency_AddRemove(t *testing.T) {
	t.Parallel()

	tw := newTW()
	defer tw.Stop()

	const goroutines = 20
	const ops = 50

	var wg sync.WaitGroup
	wg.Add(goroutines)
	for g := range goroutines {
		go func() {
			defer wg.Done()
			for range ops {
				id := "timer"
				if g%2 == 0 {
					tw.Add(id, 100*time.Millisecond, false, func() {})
				} else {
					tw.Remove(id)
				}
			}
		}()
	}
	wg.Wait()
}

func TestConcurrency_UpdateTTL(t *testing.T) {
	t.Parallel()

	tw := newTW()
	defer tw.Stop()

	tw.Add("shared", 500*time.Millisecond, false, func() {})

	var wg sync.WaitGroup
	for range 10 {
		wg.Go(func() {
			tw.UpdateTTL("shared", 300*time.Millisecond)
		})
	}
	wg.Wait()
}

func TestConcurrency_ManyTimers(t *testing.T) {
	t.Parallel()

	tw := newTW()
	defer tw.Stop()

	const n = 100
	var fired atomic.Int64

	for i := range n {
		tw.Add(fmt.Sprintf("timer-%d", i), time.Duration(20+i%30)*time.Millisecond, false, func() {
			fired.Add(1)
		})
	}
	time.Sleep(300 * time.Millisecond)

	f := fired.Load()
	if f < int64(n/2) {
		t.Errorf("expected most timers to fire, got %d/%d", f, n)
	}
}

func TestResetTimer(t *testing.T) {
	t.Parallel()

	timer := &Timer{
		id:      "abc",
		task:    func() {},
		rounds:  5,
		slotIdx: 3,
		repeat:  true,
	}
	resetTimer(timer)

	if timer.id != "" {
		t.Errorf("id should be empty, got %q", timer.id)
	}
	if timer.task != nil {
		t.Error("task should be nil")
	}
	if timer.rounds != 0 {
		t.Errorf("rounds should be 0, got %d", timer.rounds)
	}
	if timer.slotIdx != 0 {
		t.Errorf("slotIdx should be 0, got %d", timer.slotIdx)
	}
	if timer.repeat {
		t.Error("repeat should be false")
	}
}

func TestRemove_IndexUpdatedForSwappedTimer(t *testing.T) {
	t.Parallel()

	tw := newTW()
	defer tw.Stop()

	tw.Add("first", 300*time.Millisecond, false, func() {})
	tw.Add("second", 300*time.Millisecond, false, func() {})
	tw.Add("third", 300*time.Millisecond, false, func() {})
	tw.Remove("first")

	active := tw.ListActive()
	if _, ok := active["second"]; !ok {
		t.Error("'second' should still be active")
	}
	if _, ok := active["third"]; !ok {
		t.Error("'third' should still be active after index update")
	}

	tw.Remove("third")
	active = tw.ListActive()
	if _, ok := active["third"]; ok {
		t.Error("'third' should be gone after second Remove")
	}
}

func TestAdd_Overwrite_IndexUpdatedForSwappedTimer(t *testing.T) {
	t.Parallel()

	tw := newTW()
	defer tw.Stop()

	tw.mu.Lock()

	targetSlot := 5
	tA := &Timer{id: "A", interval: 50 * time.Millisecond, slotIdx: 0}
	tB := &Timer{id: "B", interval: 50 * time.Millisecond, slotIdx: 1}
	tw.slots[targetSlot].timers = append(tw.slots[targetSlot].timers, tA, tB)
	tw.index["A"] = [2]int{targetSlot, 0}
	tw.index["B"] = [2]int{targetSlot, 1}

	tw.mu.Unlock()

	tw.Add("A", 50*time.Millisecond, false, func() {})
	tw.Remove("B")

	active := tw.ListActive()
	if _, ok := active["B"]; ok {
		t.Error("'B' should be gone after Remove")
	}
	if _, ok := active["A"]; !ok {
		t.Error("'A' should still be active after re-Add")
	}
}

func TestUpdateTTL_IndexUpdatedForSwappedTimer(t *testing.T) {
	t.Parallel()

	tw := newTW()
	defer tw.Stop()

	tw.Add("a", 300*time.Millisecond, false, func() {})
	tw.Add("b", 300*time.Millisecond, false, func() {})
	tw.Add("c", 300*time.Millisecond, false, func() {})
	tw.UpdateTTL("a", 400*time.Millisecond)

	active := tw.ListActive()
	for _, id := range []string{"a", "b", "c"} {
		if _, ok := active[id]; !ok {
			t.Errorf("timer %q should be active after UpdateTTL reshuffled the slot", id)
		}
	}

	tw.Remove("b")
	tw.Remove("c")
	active = tw.ListActive()
	if len(active) != 1 {
		t.Errorf("expected 1 active timer, got %d", len(active))
	}
}

func TestTimer_MultiRound_FiresOnlyAfterFullInterval(t *testing.T) {
	t.Parallel()

	tw := NewTimingWheel(20*time.Millisecond, 5)
	defer tw.Stop()

	fired := make(chan time.Time, 1)
	start := time.Now()
	tw.Add("long", 150*time.Millisecond, false, func() {
		fired <- time.Now()
	})

	select {
	case at := <-fired:
		elapsed := at.Sub(start)
		if elapsed < 100*time.Millisecond {
			t.Errorf("fired too early: %v (rounds branch not traversed)", elapsed)
		}
	case <-time.After(500 * time.Millisecond):
		t.Error("multi-round timer never fired")
	}
}

func TestDispatch_FallbackToGoroutine(t *testing.T) {
	t.Parallel()

	tw := newTW()
	defer tw.Stop()

	const total = 300
	var wg sync.WaitGroup
	wg.Add(total)

	blocker := make(chan struct{})
	for range workerCount {
		tw.taskCh <- func() { <-blocker }
	}

	for range total {
		tw.dispatch(func() { wg.Done() })
	}
	close(blocker)

	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Error("not all dispatched tasks completed in time")
	}
}

func TestAdd_ZeroInterval_UsesMinimumOneTick(t *testing.T) {
	t.Parallel()

	tw := newTW()
	defer tw.Stop()

	fired := make(chan struct{}, 1)
	tw.Add("t1", 0, false, func() {
		fired <- struct{}{}
	})

	select {
	case <-fired:
	case <-time.After(200 * time.Millisecond):
		t.Error("zero-interval timer should fire quickly (using 1-tick minimum)")
	}
}
