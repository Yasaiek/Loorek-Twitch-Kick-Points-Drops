package main

import (
	"regexp"
	"sort"
	"strings"
	"time"
)

type Drop struct {
	ID       string
	Name     string
	Req, Cur int64
	Claimed  bool
	HasSelf  bool
	ClaimID  string
	Pre      []string
	Benefits []string
	Img      string
	Start    time.Time
	End      time.Time
	Day      int       // reward campaigns: which round of a repeating (daily) group this is, 0 = does not repeat
	Later    bool      // reward campaigns: a later stage, opens after the stages already started
	Days     int       // reward campaigns, daily rewards: how many days in all (0 = Twitch does not say)
	Done     int       // reward campaigns, daily rewards: days already collected
	Until    time.Time // reward campaigns, daily rewards: today's round has to be finished by then
	Next     time.Time // reward campaigns: this round is done, the next one opens then
}

type ACL struct{ ID, Login, Name string }

type Campaign struct {
	ID, Name string
	Game     GameRef
	Status   string
	Start    time.Time
	End      time.Time
	Linked   bool
	LinkURL  string
	Channels []ACL
	Drops    []*Drop
	Quest    bool      // one of Twitch's own reward campaigns (see quests.go)
	Cats     []GameRef // reward campaigns that count in several categories: all of them (Game is then the campaign itself)
}

type farm struct {
	game     GameRef
	ch       *chanRT
	campIDs  map[string]bool
	dropID   string
	started  time.Time
	lastProg time.Time
	lastPoll time.Time
	lastCur  int64
	total    int64           // minutes seen on the account for this game at the last inventory read
	solo     bool            // points watching is paused so that nothing competes with this drop
	told     map[string]bool // drops whose completion was already announced during this farm
	quest    time.Time       // a reward campaign is among the farmed ones: when its progress was last read
}

type dropsState struct {
	camps     map[string]*Campaign
	fetchedAt time.Time
	nextFetch time.Time
	fetchErr  string
	force     bool
	cur       *farm
	nextPick  time.Time
	badCh     map[string]time.Time // channel id -> not before
	gameWait  map[string]time.Time // game id -> not before
	gameNote  map[string]string    // game id -> why nothing is farmed (nostreams)
	claimFail map[string]int
	claimedAt map[string]time.Time // drop id -> when it was last claimed here
	stalls    map[string]int       // game id -> how many times in a row watching brought nothing
	slugs     map[string]string
}

func (d *dropsState) init() {
	d.camps = map[string]*Campaign{}
	d.badCh = map[string]time.Time{}
	d.gameWait = map[string]time.Time{}
	d.gameNote = map[string]string{}
	d.claimFail = map[string]int{}
	d.claimedAt = map[string]time.Time{}
	d.stalls = map[string]int{}
	d.slugs = map[string]string{}
}

func (d *Drop) finished() bool { return d.Claimed || (d.Req > 0 && d.Cur >= d.Req) }

func (c *Campaign) active(now time.Time) bool { return !now.Before(c.Start) && now.Before(c.End) }

func (c *Campaign) drop(id string) *Drop {
	for _, d := range c.Drops {
		if d.ID == id {
			return d
		}
	}
	return nil
}

// earnable lists the drops that can make progress right now.
func (c *Campaign) earnable(now time.Time) []*Drop {
	if !c.active(now) || !c.Linked {
		return nil
	}
	var out []*Drop
	for _, d := range c.Drops {
		if d.Req <= 0 || d.finished() || d.Later {
			continue
		}
		if (!d.Start.IsZero() && now.Before(d.Start)) || (!d.End.IsZero() && !now.Before(d.End)) {
			continue
		}
		ok := true
		for _, p := range d.Pre {
			if pd := c.drop(p); pd != nil && !pd.Claimed {
				ok = false
			}
		}
		if ok {
			out = append(out, d)
		}
	}
	return out
}

var reDims = regexp.MustCompile(`-(\{width\}x\{height\}|\d+x\d+)\.`)

func boxArt(u string) string {
	if u == "" {
		return ""
	}
	return reDims.ReplaceAllString(u, "-144x192.")
}

var reNonWord = regexp.MustCompile(`[^\p{L}\p{N}_]+`)

func guessSlug(name string) string {
	s := strings.ReplaceAll(strings.ToLower(name), "'", "")
	s = reNonWord.ReplaceAllString(s, "-")
	return strings.Trim(s, "-")
}

func gameRef(g any) GameRef {
	name := str(dig(g, "displayName"))
	if name == "" {
		name = str(dig(g, "name"))
	}
	return GameRef{ID: str(dig(g, "id")), Slug: str(dig(g, "slug")), Name: name, Art: boxArt(str(dig(g, "boxArtURL")))}
}

// buildCampaign merges what the inventory (progress), the campaign details (channels, rewards)
// and the campaign list (the only one that always carries the game cover) say.
func buildCampaign(inv, det, dash any, claimed map[string]time.Time) *Campaign {
	pick := func(path ...string) any {
		if v := dig(inv, path...); v != nil {
			return v
		}
		return dig(det, path...)
	}
	game := pick("game")
	if game == nil {
		return nil
	}
	c := &Campaign{
		ID: str(pick("id")), Name: str(pick("name")), Status: str(pick("status")),
		Start: parseTime(pick("startAt")), End: parseTime(pick("endAt")), LinkURL: str(pick("accountLinkURL")),
	}
	c.Game = gameRef(game)
	if dg := dig(det, "game"); dg != nil { // details carry the fuller game record
		g := gameRef(dg)
		if c.Game.Slug == "" {
			c.Game.Slug = g.Slug
		}
		if c.Game.Art == "" {
			c.Game.Art = g.Art
		}
	}
	if c.Game.Art == "" {
		c.Game.Art = boxArt(str(dig(dash, "game", "boxArtURL")))
	}
	if c.Game.Art == "" && c.Game.ID != "" { // Twitch serves covers by game id as well
		c.Game.Art = "https://static-cdn.jtvnw.net/ttv-boxart/" + c.Game.ID + "-144x192.jpg"
	}
	if s := pick("self"); s != nil {
		c.Linked = boolean(dig(s, "isAccountConnected"))
	}
	allow := pick("allow")
	if allow != nil && (dig(allow, "isEnabled") == nil || boolean(dig(allow, "isEnabled"))) {
		for _, ch := range list(dig(allow, "channels")) {
			c.Channels = append(c.Channels, ACL{ID: str(dig(ch, "id")), Login: strings.ToLower(str(dig(ch, "name"))), Name: str(dig(ch, "displayName"))})
		}
	}
	detDrops := map[string]any{}
	for _, d := range list(dig(det, "timeBasedDrops")) {
		detDrops[str(dig(d, "id"))] = d
	}
	raw := list(dig(inv, "timeBasedDrops"))
	if len(raw) == 0 {
		raw = list(dig(det, "timeBasedDrops"))
	}
	for _, r := range raw {
		id := str(dig(r, "id"))
		other := detDrops[id]
		get := func(k string) any {
			if v := dig(r, k); v != nil {
				return v
			}
			return dig(other, k)
		}
		d := &Drop{ID: id, Name: str(get("name")), Req: num(get("requiredMinutesWatched")), Start: parseTime(get("startAt")), End: parseTime(get("endAt"))}
		for _, p := range list(get("preconditionDrops")) {
			d.Pre = append(d.Pre, str(dig(p, "id")))
		}
		edges := list(dig(r, "benefitEdges"))
		if len(edges) == 0 {
			edges = list(dig(other, "benefitEdges"))
		}
		for _, be := range edges {
			b := dig(be, "benefit")
			if id := str(dig(b, "id")); id != "" {
				d.Benefits = append(d.Benefits, id)
			}
			if d.Img == "" {
				d.Img = str(dig(b, "imageAssetURL"))
			}
			if d.Name == "" {
				d.Name = str(dig(b, "name"))
			}
		}
		if self := dig(r, "self"); self != nil {
			d.HasSelf = true
			d.Cur = num(dig(self, "currentMinutesWatched"))
			d.Claimed = boolean(dig(self, "isClaimed"))
			d.ClaimID = str(dig(self, "dropInstanceID"))
		} else {
			// no progress record: a reward awarded while this drop was open means it has been claimed
			found, within := 0, 0
			for _, b := range d.Benefits {
				if at, ok := claimed[b]; ok {
					found++
					if !at.Before(d.Start) && at.Before(d.End) {
						within++
					}
				}
			}
			if found > 0 && found == within {
				d.Claimed = true
			}
		}
		if d.Claimed && d.Cur < d.Req {
			d.Cur = d.Req
		}
		c.Drops = append(c.Drops, d)
	}
	return c
}

// fetchDrops reads the account's inventory and every open campaign.
func (e *Engine) fetchDrops(now time.Time) bool {
	_, uid, _, _ := e.c.auth()
	inv, err := e.c.gqlOne("Inventory", map[string]any{"fetchRewardCampaigns": false})
	if err != nil {
		e.gqlErr("inventory", err)
		e.setDropsErr(err)
		return false
	}
	dash, err := e.c.gqlOne("ViewerDropsDashboard", map[string]any{"fetchRewardCampaigns": false})
	if err != nil {
		e.gqlErr("campaigns", err)
		e.setDropsErr(err)
		return false
	}
	inventory := dig(inv, "currentUser", "inventory")
	claimed := map[string]time.Time{}
	for _, b := range list(dig(inventory, "gameEventDrops")) {
		claimed[str(dig(b, "id"))] = parseTime(dig(b, "lastAwardedAt"))
	}
	inProgress := map[string]any{}
	for _, c := range list(dig(inventory, "dropCampaignsInProgress")) {
		inProgress[str(dig(c, "id"))] = c
	}
	var ids []string
	listed := map[string]any{}
	for _, c := range list(dig(dash, "currentUser", "dropCampaigns")) {
		listed[str(dig(c, "id"))] = c
		if st := str(dig(c, "status")); st == "ACTIVE" || st == "UPCOMING" {
			ids = append(ids, str(dig(c, "id")))
		}
	}
	ops := make([]gqlOp, len(ids))
	for i, id := range ids {
		ops[i] = gqlOp{"DropCampaignDetails", map[string]any{"channelLogin": uid, "dropID": id}}
	}
	details, err := e.c.gqlBatch(ops)
	e.gqlErr("campaign details", err)
	if details == nil {
		e.setDropsErr(err)
		return false
	}
	camps := map[string]*Campaign{}
	missing := 0
	for i, id := range ids {
		var det any
		if details[i] != nil {
			det = dig(details[i], "user", "dropCampaign")
		}
		if det == nil {
			missing++
		}
		if c := buildCampaign(inProgress[id], det, listed[id], claimed); c != nil && c.ID != "" {
			camps[c.ID] = c
		}
	}
	for id, invC := range inProgress {
		if camps[id] == nil {
			if c := buildCampaign(invC, nil, listed[id], claimed); c != nil && c.ID != "" {
				camps[c.ID] = c
			}
		}
	}
	// Twitch's own reward campaigns (daily chests, badge collections) come from a separate place
	quests, ok := e.fetchQuests(now, uid)
	e.mu.Lock()
	if !ok { // not read this time: keep the ones already known
		for id, c := range e.drops.camps {
			if c.Quest && camps[id] == nil {
				camps[id] = c
			}
		}
	}
	e.mu.Unlock()
	e.mu.Lock()
	for _, q := range quests {
		e.tellDays(e.drops.camps[q.ID], q, now)
		camps[q.ID] = q
	}
	first := e.drops.fetchedAt.IsZero()
	e.drops.camps = camps
	e.drops.fetchedAt, e.drops.fetchErr = now, ""
	e.mu.Unlock()
	if first || missing > 0 {
		e.log.Info("L.camps", len(camps), len(inProgress))
	}
	if missing > 0 {
		e.log.Debug("campaign details missing for %d of %d", missing, len(ids))
	}
	return true
}

func (e *Engine) setDropsErr(err error) {
	e.mu.Lock()
	if err != nil {
		e.drops.fetchErr = err.Error()
	}
	e.mu.Unlock()
}

// claimDrops claims every finished, unclaimed drop (only for linked game accounts).
func (e *Engine) claimDrops(now time.Time) {
	_, uid, _, _ := e.c.auth()
	type job struct {
		c *Campaign
		d *Drop
	}
	var jobs []job
	e.mu.Lock()
	for _, c := range e.drops.camps {
		if !c.Linked || now.After(c.End.Add(24*time.Hour)) {
			continue
		}
		for _, d := range c.Drops {
			if d.Claimed || !d.HasSelf || d.Req <= 0 || d.Cur < d.Req || e.drops.claimFail[d.ID] >= 3 {
				continue
			}
			if c.Quest && d.ClaimID == "" {
				continue // a reward campaign: Twitch grants it by itself unless it says there is something to claim
			}
			if at, ok := e.drops.claimedAt[d.ID]; ok && now.Sub(at) < iv(30*time.Minute) {
				d.Claimed = true // claimed a moment ago; the account has not caught up yet - never ask twice
				continue
			}
			jobs = append(jobs, job{c, d})
		}
	}
	e.mu.Unlock()
	for _, j := range jobs {
		id := j.d.ClaimID
		if id == "" {
			id = uid + "#" + j.c.ID + "#" + j.d.ID
		}
		data, err := e.c.gqlOne("DropsPage_ClaimDropRewards", map[string]any{"input": map[string]any{"dropInstanceID": id}})
		status := str(dig(data, "claimDropRewards", "status"))
		e.mu.Lock()
		if err == nil && (status == "ELIGIBLE_FOR_ALL" || status == "DROP_INSTANCE_ALREADY_CLAIMED") {
			j.d.Claimed = true
			e.drops.claimedAt[j.d.ID] = now
			if next := now.Add(iv(10 * time.Second)); j.c.Quest && next.Before(e.drops.nextFetch) {
				e.drops.nextFetch = next // a daily reward: read when its next round opens
			}
			e.mu.Unlock()
			e.log.OK("L.claimed", j.d.Name, j.c.Game.Name)
			continue
		}
		e.drops.claimFail[j.d.ID]++
		e.mu.Unlock()
		e.gqlErr("claim drop", err)
		e.log.Warn("L.claimFail", j.d.Name, j.c.Game.Name, status)
	}
}

func sameGame(a, b GameRef) bool {
	if a.ID != "" && b.ID != "" {
		return a.ID == b.ID
	}
	return a.Slug != "" && a.Slug == b.Slug
}

// gameCamps returns a game's campaigns that are running now. Call with e.mu held.
func (e *Engine) gameCamps(g GameRef, now time.Time) []*Campaign {
	var out []*Campaign
	for _, c := range e.drops.camps {
		if sameGame(c.Game, g) && c.active(now) {
			out = append(out, c)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].End.Before(out[j].End) })
	return out
}

// gameStatus: ready | claim | done | nolink | nocamp | upcoming | blocked. Call with e.mu held.
func (e *Engine) gameStatus(g GameRef, now time.Time) string {
	camps := e.gameCamps(g, now)
	if len(camps) == 0 {
		for _, c := range e.drops.camps {
			if sameGame(c.Game, g) && now.Before(c.Start) {
				return "upcoming"
			}
		}
		return "nocamp"
	}
	timed, claimed, toClaim, unlinked, today := 0, 0, 0, false, false
	for _, c := range camps {
		if len(c.earnable(now)) > 0 {
			return "ready"
		}
		for _, d := range c.Drops {
			if d.Req <= 0 {
				continue
			}
			timed++
			if d.Claimed && now.Before(d.Next) {
				today = true // a daily reward: done for today, the next round opens later
			}
			switch {
			case d.Claimed:
				claimed++
			case d.Cur >= d.Req:
				toClaim++
			default:
				if !c.Linked {
					unlinked = true
				}
			}
		}
	}
	switch {
	case toClaim > 0:
		return "claim"
	case today:
		return "today"
	case timed > 0 && claimed == timed:
		return "done"
	case unlinked:
		return "nolink"
	}
	return "blocked"
}

// candidates: queue first, then tracked games. Call with e.mu held.
func (e *Engine) candidates() []GameRef {
	out := append([]GameRef(nil), e.set.Queue...)
	for _, t := range e.set.Tracked {
		dup := false
		for _, q := range out {
			if sameGame(q, t) {
				dup = true
			}
		}
		if !dup {
			out = append(out, t)
		}
	}
	return out
}

func (e *Engine) dropsStep(now time.Time) {
	e.mu.Lock()
	on := e.set.DropsOn
	need := e.drops.force || !now.Before(e.drops.nextFetch)
	auto := e.set.AutoClaim
	e.mu.Unlock()
	if need {
		ok := e.fetchDrops(now)
		e.mu.Lock()
		e.drops.force = false
		switch {
		case !ok && e.drops.fetchedAt.IsZero(): // nothing to show yet (Twitch timed out): do not make the user wait long
			e.drops.nextFetch = now.Add(iv(20 * time.Second))
		case !ok:
			e.drops.nextFetch = now.Add(iv(2 * time.Minute))
		case e.drops.cur != nil:
			e.drops.nextFetch = now.Add(iv(10 * time.Minute))
		default:
			e.drops.nextFetch = now.Add(iv(30 * time.Minute))
		}
		if ok { // a daily reward that reopens before the next reading: read again right when it does
			for _, c := range e.drops.camps {
				for _, d := range c.Drops {
					if at := d.Next.Add(5 * time.Second); !d.Next.IsZero() && at.After(now) && at.Before(e.drops.nextFetch) {
						e.drops.nextFetch = at
					}
				}
			}
		}
		e.mu.Unlock()
		if ok && auto {
			e.claimDrops(now)
		}
		if ok {
			e.mu.Lock()
			if f := e.drops.cur; f != nil { // progress confirmed by the account also counts as "not stalled"
				var total int64
				for _, c := range e.gameCamps(f.game, now) {
					for _, d := range c.Drops {
						total += d.Cur
					}
				}
				if total > f.total {
					f.lastProg = now
					delete(e.drops.stalls, f.game.ID)
				}
				f.total = total
			}
			e.mu.Unlock()
		}
	}
	if !on {
		e.mu.Lock()
		e.drops.cur = nil
		e.mu.Unlock()
		return
	}

	e.mu.Lock()
	if e.drops.fetchedAt.IsZero() {
		e.mu.Unlock()
		return
	}
	// which game should be farmed now
	var top *GameRef
	for _, g := range e.candidates() {
		if e.gameStatus(g, now) == "ready" && !now.Before(e.drops.gameWait[g.ID]) {
			g := g
			top = &g
			break
		}
	}
	cur := e.drops.cur
	if cur != nil {
		switch {
		case top == nil || !sameGame(*top, cur.game):
			switch st := e.gameStatus(cur.game, now); {
			case st == "today":
				e.log.OK("L.todayDone", cur.game.Name, logWhen(e.nextRound(cur.game, now), now))
			case st != "ready":
				e.log.OK("L.gameDone", cur.game.Name)
			}
			e.drops.cur, cur = nil, nil
		case !cur.ch.Online:
			e.log.Info("L.streamEnded", cur.game.Name, cur.ch.Name)
			e.drops.cur, cur = nil, nil
			e.drops.nextPick = now
		case now.Sub(cur.lastProg) > e.stallLimit(cur) && !cur.solo && len(e.slots) > 1:
			// Twitch advances drops on one channel at a time: the second watched channel may be taking the progress
			cur.solo, cur.lastProg = true, now
			e.log.Warn("L.solo", cur.game.Name)
		case now.Sub(cur.lastProg) > e.stallLimit(cur):
			e.drops.stalls[cur.game.ID]++
			if !cur.quest.IsZero() && e.drops.stalls[cur.game.ID] >= 2 {
				// A reward campaign that does not move on two streams in a row: Twitch wants something that
				// watching alone does not give. Leave it for a few hours instead of watching for nothing.
				e.log.Warn("L.stuck", cur.game.Name, stuckHours)
				e.drops.gameWait[cur.game.ID] = now.Add(iv(stuckHours * time.Hour))
				e.drops.gameNote[cur.game.ID] = "stuck"
				delete(e.drops.stalls, cur.game.ID)
				top = nil // it was this step's choice: the next step picks the next game
			} else {
				e.log.Warn("L.stall", cur.game.Name, cur.ch.Name)
				e.drops.badCh[cur.ch.ID] = now.Add(iv(30 * time.Minute))
			}
			e.drops.cur, cur = nil, nil
			e.drops.nextPick = now
		}
	}
	pick := cur == nil && top != nil && !now.Before(e.drops.nextPick)
	e.mu.Unlock()

	if pick {
		ch, campIDs := e.findDropChannel(*top, now)
		e.mu.Lock()
		if ch != nil {
			e.drops.cur = &farm{game: *top, ch: ch, campIDs: campIDs, started: now, lastProg: now, lastPoll: now.Add(-iv(30 * time.Second))}
			for id := range campIDs {
				if c := e.drops.camps[id]; c != nil && c.Quest {
					e.drops.cur.quest = now
				}
			}
			delete(e.drops.gameNote, top.ID)
			if next := now.Add(iv(10 * time.Minute)); next.Before(e.drops.nextFetch) {
				e.drops.nextFetch = next // while farming the account is read more often
			}
			e.log.OK("L.farm", top.Name, ch.Name)
		} else {
			e.drops.gameWait[top.ID] = now.Add(iv(5 * time.Minute))
			e.drops.gameNote[top.ID] = "nostreams"
			e.drops.nextPick = now.Add(iv(10 * time.Second))
			e.log.Info("L.noStreams", top.Name)
		}
		e.mu.Unlock()
		return
	}
	if cur != nil && now.Sub(cur.lastPoll) >= iv(time.Minute) {
		e.pollDrop(cur, now)
	}
	if cur != nil && !cur.quest.IsZero() && now.Sub(cur.quest) >= iv(2*time.Minute) {
		e.pollQuests(cur, now)
	}
}

// pollQuests re-reads only Twitch's reward campaigns while one of them is being farmed. Twitch does not
// report a running reward the way it reports a drop, and the full account reading is too heavy to repeat
// every couple of minutes; this one is two small requests and keeps the progress on screen moving.
func (e *Engine) pollQuests(f *farm, now time.Time) {
	f.quest = now
	_, uid, _, _ := e.c.auth()
	quests, ok := e.fetchQuests(now, uid)
	if !ok {
		return
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	for _, q := range quests {
		e.tellDays(e.drops.camps[q.ID], q, now)
		e.drops.camps[q.ID] = q
		for _, d := range q.Drops {
			if !d.Claimed && d.HasSelf && d.Req > 0 && d.Cur >= d.Req && now.Add(iv(5*time.Second)).Before(e.drops.nextFetch) {
				e.drops.nextFetch = now.Add(iv(5 * time.Second)) // finished: the full reading confirms and claims it
			}
		}
	}
	var total int64
	for _, c := range e.gameCamps(f.game, now) {
		for _, d := range c.Drops {
			total += d.Cur
		}
	}
	if total > f.total {
		f.lastProg = now
		delete(e.drops.stalls, f.game.ID)
	}
	f.total = total
}

// tellDays writes to the journal when a day of a daily reward has been collected. Call with e.mu held.
func (e *Engine) tellDays(old, q *Campaign, now time.Time) {
	if old == nil {
		return
	}
	for _, d := range q.Drops {
		if d.Days == 0 && d.Done == 0 && d.Next.IsZero() {
			continue // not a daily reward
		}
		was := old.drop(d.ID)
		if was == nil || was.finished() || !d.finished() {
			continue
		}
		e.log.OK("L.dayGot", q.Game.Name, d.Name, max(d.Day, 1))
		if d.Days > 0 && d.Done >= d.Days {
			e.log.OK("L.daysAll", q.Game.Name, d.Name)
		}
	}
}

// nextRound: when the next day of a daily reward opens for this game (zero: none waits). Call with e.mu held.
func (e *Engine) nextRound(g GameRef, now time.Time) time.Time {
	var at time.Time
	for _, c := range e.gameCamps(g, now) {
		for _, d := range c.Drops {
			if d.Claimed && now.Before(d.Next) && (at.IsZero() || d.Next.Before(at)) {
				at = d.Next
			}
		}
	}
	return at
}

// logWhen: a time for the journal - "15:00" today, "22.11 15:00" later.
func logWhen(t, now time.Time) string {
	if t.IsZero() {
		return "?"
	}
	t = t.Local()
	if y, m, d := t.Date(); y == now.Local().Year() && m == now.Local().Month() && d == now.Local().Day() {
		return t.Format("15:04")
	}
	return t.Format("02.01 15:04")
}

// stuckHours: how long a reward campaign that watching does not move is left alone.
const stuckHours = 6

// stallLimit: how long without progress before the stream is changed. When Twitch does not report the
// running drop at all, progress is only visible in the inventory (read every 10 min), so wait longer.
func (e *Engine) stallLimit(f *farm) time.Duration {
	m := e.set.StallMin
	if f.dropID == "" && m < 25 {
		m = 25
	}
	return iv(time.Duration(m) * time.Minute)
}

// pollDrop asks Twitch which drop is progressing on the watched channel and how far it is.
func (e *Engine) pollDrop(f *farm, now time.Time) {
	f.lastPoll = now
	data, err := e.c.gqlOne("DropCurrentSessionContext", map[string]any{"channelID": f.ch.ID, "channelLogin": ""})
	if err != nil {
		e.gqlErr("drop progress", err)
		return
	}
	s := dig(data, "currentUser", "dropCurrentSession")
	e.mu.Lock()
	defer e.mu.Unlock()
	recheck := func() { // the running drop is gone or changed: it was probably finished - read the account
		if next := now.Add(iv(10 * time.Second)); next.Before(e.drops.nextFetch) && now.Sub(e.drops.fetchedAt) > iv(time.Minute) {
			e.drops.nextFetch = next
		}
	}
	if s == nil {
		if f.dropID != "" {
			e.log.Debug("drop session on %s: none (was %s)", f.ch.Login, f.dropID)
			f.dropID = ""
			recheck()
		}
		return
	}
	id, cur := str(dig(s, "dropID")), num(dig(s, "currentMinutesWatched"))
	if id != f.dropID || cur != f.lastCur {
		e.log.Debug("drop session on %s: %s %d/%d", f.ch.Login, id, cur, num(dig(s, "requiredMinutesWatched")))
	}
	var camp *Campaign
	var drop *Drop
	for _, c := range e.drops.camps {
		if d := c.drop(id); d != nil {
			camp, drop = c, d
			break
		}
	}
	// Twitch sometimes counts the minutes on a reward that cannot be earned: one already granted, or a later
	// stage of a reward campaign the account has not opened. Such minutes lead nowhere - they are no progress.
	dead := drop != nil && (drop.Claimed || drop.Later || (camp.Quest && drop.Cur >= drop.Req))
	if drop != nil && camp.Quest && !dead {
		// Twitch's session adds the minutes of every day together (1454/60): it only shows that the
		// minutes run. The progress itself comes from the account (pollQuests).
		if id == f.dropID && cur > f.lastCur {
			f.lastProg = now
			delete(e.drops.stalls, f.game.ID)
		}
		f.dropID, f.lastCur = id, cur
		return
	}
	if dead {
		if f.dropID != id {
			e.log.Debug("drop session on %s: %s cannot be earned - not counted as progress", f.ch.Login, id)
		}
		f.dropID, f.lastCur = id, cur
		return
	}
	if id != f.dropID || cur > f.lastCur {
		f.lastProg = now
		delete(e.drops.stalls, f.game.ID)
	}
	if f.dropID != "" && id != f.dropID {
		recheck()
	}
	f.dropID, f.lastCur = id, cur
	if drop != nil {
		if cur > drop.Cur {
			drop.Cur = cur
		}
		drop.HasSelf = true
		if drop.Req > 0 && drop.Cur >= drop.Req && !f.told[id] {
			if f.told == nil {
				f.told = map[string]bool{}
			}
			f.told[id] = true // said once: the account reading replaces the record, and the session keeps reporting it
			e.log.OK("L.dropDone", camp.Game.Name, drop.Name)
			e.drops.nextFetch = now.Add(iv(10 * time.Second)) // confirm on the account, then claim
		}
	}
}

// findDropChannel looks for a live channel on which the game's open campaigns really run.
func (e *Engine) findDropChannel(g GameRef, now time.Time) (*chanRT, map[string]bool) {
	e.mu.Lock()
	campIDs := map[string]bool{}
	acl := map[string]ACL{}
	allACL := true
	questOnly := true  // only Twitch's reward campaigns are open for this game
	var dirs []GameRef // the categories whose streams count: the game itself, or a reward campaign's categories
	for _, c := range e.gameCamps(g, now) {
		if len(c.earnable(now)) == 0 {
			continue
		}
		campIDs[c.ID] = true
		dirs = append(dirs, c.Cats...)
		if !c.Quest {
			questOnly = false
		}
		if len(c.Channels) == 0 {
			allACL = false
		}
		for _, a := range c.Channels {
			if a.Login != "" {
				acl[a.Login] = a
			}
		}
		if g.Slug == "" {
			g.Slug = c.Game.Slug
		}
	}
	bad := map[string]time.Time{}
	for k, v := range e.drops.badCh {
		bad[k] = v
	}
	slug := g.Slug
	if s := e.drops.slugs[g.ID]; s != "" {
		slug = s
	}
	e.mu.Unlock()
	if len(campIDs) == 0 {
		return nil, nil
	}

	var cands []*chanRT
	if len(acl) > 0 { // channels named by the campaigns
		var chs []*chanRT
		for _, a := range acl {
			chs = append(chs, &chanRT{Login: a.Login, ID: a.ID, Name: firstNonEmpty(a.Name, a.Login)})
			if len(chs) >= 80 {
				break
			}
		}
		e.streamInfo(chs)
		for _, ch := range chs {
			if ch.Online && (ch.GameID == "" || g.ID == "" || ch.GameID == g.ID) {
				cands = append(cands, ch)
			}
		}
	}
	if len(dirs) > 0 { // a reward campaign counting in several categories: the first one with drops-enabled streams
		if len(dirs) > 12 {
			dirs = dirs[:12]
		}
		for _, cat := range dirs {
			s := firstNonEmpty(cat.Slug, guessSlug(cat.Name))
			cands = append(cands, e.dropStreams(s, cat)...)
			if len(cands) > 0 {
				break
			}
		}
	} else if !allACL { // any channel of the game with drops switched on
		if slug == "" {
			if data, err := e.c.gqlOne("DirectoryGameRedirect", map[string]any{"name": g.Name}); err == nil {
				slug = str(dig(data, "game", "slug"))
			}
			if slug == "" {
				slug = guessSlug(g.Name)
			}
			e.mu.Lock()
			e.drops.slugs[g.ID] = slug
			e.mu.Unlock()
		}
		cands = append(cands, e.dropStreams(slug, g)...)
	}
	sort.SliceStable(cands, func(i, j int) bool { return cands[i].Viewers > cands[j].Viewers })
	var first *chanRT
	checked := 0
	for _, ch := range cands {
		if until, ok := bad[ch.ID]; ok && now.Before(until) {
			continue
		}
		if first == nil {
			first = ch
		}
		if questOnly {
			// A reward campaign counts on the category's streams with drops on and is not listed per
			// channel, so there is nothing to verify: take the biggest one. A stream that brings no
			// progress is dropped later by the usual "stalled" rule.
			return e.adopt(ch), campIDs
		}
		if checked >= 6 {
			break
		}
		checked++
		data, err := e.c.gqlOne("DropsHighlightService_AvailableDrops", map[string]any{"channelID": ch.ID})
		if err != nil {
			e.gqlErr("available drops", err)
			continue
		}
		for _, vc := range list(dig(data, "channel", "viewerDropCampaigns")) {
			if campIDs[str(dig(vc, "id"))] {
				return e.adopt(ch), campIDs
			}
		}
	}
	if first != nil && checked == 0 {
		return e.adopt(first), campIDs
	}
	return nil, nil
}

// dropStreams lists the live channels of a category that have drops switched on.
func (e *Engine) dropStreams(slug string, g GameRef) []*chanRT {
	data, err := e.c.gqlOne("DirectoryPage_Game", map[string]any{
		"limit": 30, "slug": slug, "imageWidth": 50, "includeCostreaming": false, "sortTypeIsRecency": false,
		"options": map[string]any{
			"broadcasterLanguages": []string{}, "freeformTags": nil, "includeRestricted": []string{"SUB_ONLY_LIVE"},
			"recommendationsContext": map[string]any{"platform": "web"}, "sort": "RELEVANCE",
			"systemFilters": []string{"DROPS_ENABLED"}, "tags": []string{}, "requestID": "JIRA-VXP-2397",
		},
	})
	e.gqlErr("directory "+slug, err)
	var out []*chanRT
	for _, edge := range list(dig(data, "game", "streams", "edges")) {
		n := dig(edge, "node")
		b := dig(n, "broadcaster")
		if b == nil {
			continue
		}
		ch := &chanRT{Login: strings.ToLower(str(dig(b, "login"))), ID: str(dig(b, "id")), Name: firstNonEmpty(str(dig(b, "displayName")), str(dig(b, "login"))),
			Online: true, StreamID: str(dig(n, "id")), Viewers: num(dig(n, "viewersCount")), Game: g.Name, GameID: g.ID}
		if ch.Login != "" && ch.ID != "" {
			out = append(out, ch)
		}
	}
	return out
}

// adopt reuses the listed channel record when drops are farmed on a channel that is also on the points list.
func (e *Engine) adopt(ch *chanRT) *chanRT {
	e.mu.Lock()
	defer e.mu.Unlock()
	if listed := e.chans[ch.Login]; listed != nil {
		listed.ID, listed.Online, listed.StreamID = ch.ID, true, ch.StreamID
		if listed.GameID == "" {
			listed.Game, listed.GameID = ch.Game, ch.GameID
		}
		return listed
	}
	return ch
}

func firstNonEmpty(a ...string) string {
	for _, s := range a {
		if s != "" {
			return s
		}
	}
	return ""
}

// ---- operations from the panel ----

func (e *Engine) queueToggle(g GameRef) {
	e.mu.Lock()
	found := false
	out := e.set.Queue[:0]
	for _, q := range e.set.Queue {
		if sameGame(q, g) {
			found = true
			continue
		}
		out = append(out, q)
	}
	e.set.Queue = out
	if !found && (g.ID != "" || g.Slug != "") {
		e.set.Queue = append(e.set.Queue, g)
	}
	e.drops.nextPick = time.Time{}
	delete(e.drops.gameWait, g.ID)
	e.saveSettings()
	e.mu.Unlock()
	e.poke()
}

func (e *Engine) queueReorder(ids []string) {
	e.mu.Lock()
	pos := map[string]int{}
	for i, id := range ids {
		pos[id] = i + 1
	}
	sort.SliceStable(e.set.Queue, func(a, b int) bool {
		pa, pb := pos[e.set.Queue[a].ID], pos[e.set.Queue[b].ID]
		if pa == 0 {
			pa = 1 << 30
		}
		if pb == 0 {
			pb = 1 << 30
		}
		return pa < pb
	})
	e.drops.nextPick = time.Time{}
	e.saveSettings()
	e.mu.Unlock()
	e.poke()
}

func (e *Engine) trackToggle(g GameRef) {
	e.mu.Lock()
	found := false
	out := e.set.Tracked[:0]
	for _, q := range e.set.Tracked {
		if sameGame(q, g) {
			found = true
			continue
		}
		out = append(out, q)
	}
	e.set.Tracked = out
	if !found && (g.ID != "" || g.Slug != "") {
		e.set.Tracked = append(e.set.Tracked, g)
	}
	e.drops.nextPick = time.Time{}
	e.saveSettings()
	e.mu.Unlock()
	e.poke()
}
