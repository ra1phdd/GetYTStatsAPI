package timingwheel

import (
	"sync"
	"time"
)

type Port interface {
	ListActive() map[string]time.Duration
	Add(id string, interval time.Duration, repeat bool, task func())
	UpdateTTL(id string, newInterval time.Duration) bool
	Remove(id string)
	Stop()
}

type TimingWheel struct {
	tickDuration time.Duration
	mu           sync.Mutex
	slots        []slot
	index        map[string][2]int // slotPos, slotIdx
	currentPos   int
	slotsCount   int

	ticker *time.Ticker
	stopCh chan struct{}
	taskCh chan func()
}

type slot struct {
	timers []*Timer
}

func (s *slot) remove(idx int) {
	last := len(s.timers) - 1
	if idx != last {
		s.timers[idx] = s.timers[last]
		s.timers[idx].slotIdx = idx
	}
	s.timers[last] = nil
	s.timers = s.timers[:last]
}

type Timer struct {
	id       string
	rounds   int
	interval time.Duration
	task     func()
	repeat   bool

	slotIdx int
}

var timerPool = sync.Pool{
	New: func() any { return &Timer{} },
}

const workerCount = 8

func NewTimingWheel(tickDuration time.Duration, slotsCount int) *TimingWheel {
	tw := &TimingWheel{
		tickDuration: tickDuration,
		slotsCount:   slotsCount,
		slots:        make([]slot, slotsCount),
		index:        make(map[string][2]int, 64),
		ticker:       time.NewTicker(tickDuration),
		stopCh:       make(chan struct{}),
		taskCh:       make(chan func(), 256),
	}

	for i := range tw.slots {
		tw.slots[i].timers = make([]*Timer, 0)
	}

	for range workerCount {
		go tw.worker()
	}
	go tw.start()

	return tw
}

func (tw *TimingWheel) worker() {
	for {
		select {
		case fn := <-tw.taskCh:
			fn()
		case <-tw.stopCh:
			return
		}
	}
}

func (tw *TimingWheel) addLocked(t *Timer) {
	ticks := int(t.interval / tw.tickDuration)
	if ticks <= 0 {
		ticks = 1
	}
	pos := (tw.currentPos + ticks) % tw.slotsCount
	t.rounds = ticks / tw.slotsCount

	s := &tw.slots[pos]
	t.slotIdx = len(s.timers)
	s.timers = append(s.timers, t)
	tw.index[t.id] = [2]int{pos, t.slotIdx}
}

func (tw *TimingWheel) Add(id string, interval time.Duration, repeat bool, task func()) {
	t := timerPool.Get().(*Timer)
	t.id = id
	t.interval = interval
	t.task = task
	t.repeat = repeat

	tw.mu.Lock()
	if loc, ok := tw.index[id]; ok {
		tw.slots[loc[0]].remove(loc[1])

		if loc[1] < len(tw.slots[loc[0]].timers) {
			moved := tw.slots[loc[0]].timers[loc[1]]
			tw.index[moved.id] = [2]int{loc[0], loc[1]}
		}
	}
	tw.addLocked(t)
	tw.mu.Unlock()
}

func (tw *TimingWheel) Remove(id string) {
	tw.mu.Lock()
	defer tw.mu.Unlock()

	loc, ok := tw.index[id]
	if !ok {
		return
	}

	s := &tw.slots[loc[0]]
	t := s.timers[loc[1]]
	s.remove(loc[1])

	if loc[1] < len(s.timers) {
		moved := s.timers[loc[1]]
		tw.index[moved.id] = [2]int{loc[0], loc[1]}
	}
	delete(tw.index, id)

	resetTimer(t)
	timerPool.Put(t)
}

func (tw *TimingWheel) UpdateTTL(id string, newInterval time.Duration) bool {
	tw.mu.Lock()
	defer tw.mu.Unlock()

	loc, ok := tw.index[id]
	if !ok {
		return false
	}

	s := &tw.slots[loc[0]]
	t := s.timers[loc[1]]
	s.remove(loc[1])

	if loc[1] < len(s.timers) {
		moved := s.timers[loc[1]]
		tw.index[moved.id] = [2]int{loc[0], loc[1]}
	}
	delete(tw.index, id)

	if newInterval <= 0 {
		task := t.task
		resetTimer(t)
		timerPool.Put(t)

		tw.mu.Unlock()
		tw.dispatch(task)
		tw.mu.Lock()
		return true
	}

	t.interval = newInterval
	tw.addLocked(t)
	return true
}

func (tw *TimingWheel) ListActive() map[string]time.Duration {
	tw.mu.Lock()
	defer tw.mu.Unlock()

	result := make(map[string]time.Duration, len(tw.index))
	for id, loc := range tw.index {
		t := tw.slots[loc[0]].timers[loc[1]]
		slotDelta := (loc[0] - tw.currentPos + tw.slotsCount) % tw.slotsCount
		remaining := time.Duration(t.rounds)*time.Duration(tw.slotsCount)*tw.tickDuration +
			time.Duration(slotDelta)*tw.tickDuration
		result[id] = remaining
	}
	return result
}

func (tw *TimingWheel) Stop() {
	tw.ticker.Stop()
	close(tw.stopCh)
}

func (tw *TimingWheel) start() {
	for {
		select {
		case <-tw.ticker.C:
			tw.tick()
		case <-tw.stopCh:
			return
		}
	}
}

func (tw *TimingWheel) tick() {
	tw.mu.Lock()
	pos := tw.currentPos
	tw.currentPos = (tw.currentPos + 1) % tw.slotsCount

	fired := tw.slots[pos].timers
	tw.slots[pos].timers = tw.slots[pos].timers[:0]

	for _, t := range fired {
		delete(tw.index, t.id)
	}
	tw.mu.Unlock()

	for _, t := range fired {
		if t.rounds > 0 {
			t.rounds--
			nextPos := (pos + tw.slotsCount) % tw.slotsCount

			tw.mu.Lock()
			s := &tw.slots[nextPos]
			t.slotIdx = len(s.timers)
			s.timers = append(s.timers, t)
			tw.index[t.id] = [2]int{nextPos, t.slotIdx}
			tw.mu.Unlock()
			continue
		}

		timer := t
		tw.dispatch(timer.task)

		if timer.repeat {
			tw.mu.Lock()
			tw.addLocked(timer)
			tw.mu.Unlock()
		} else {
			resetTimer(timer)
			timerPool.Put(timer)
		}
	}
}

func (tw *TimingWheel) dispatch(fn func()) {
	select {
	case tw.taskCh <- fn:
	default:
		go fn()
	}
}

func resetTimer(t *Timer) {
	t.id = ""
	t.task = nil
	t.rounds = 0
	t.slotIdx = 0
	t.repeat = false
}
