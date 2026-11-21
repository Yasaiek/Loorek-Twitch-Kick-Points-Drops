package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// Twitch's own reward campaigns: daily chests, badge and emote collections ("GTAVI Launch Celebration",
// "Evolutions"). They are not in the list of drop campaigns: Twitch keeps them apart, with groups of
// rewards instead of drops. A group asks for a number of watched minutes in given categories, and a daily
// one opens again the next day. Here each of them becomes an ordinary campaign of its first category,
// with one drop per group that can be earned by watching, so the queue, tracking and farming treat it
// like any other campaign.
//
// This client has no persisted operation for them, so they are read with two plain queries.
const (
	qQuests = `query LoorekQuests { rewardCampaignsAvailableToUser { id name brand startsAt endsAt summary externalURL aboutURL isSitewide ` +
		`game { id displayName slug boxArtURL } image { image1xURL } ` +
		`rewardGroups { id unlockRequirements { minuteWatchedGoal subsGoal turboSubsGoal eventTriggerGoal } ` +
		`categories { id displayName slug boxArtURL } rewards { id name thumbnailImage { image1xURL } } } } }`
	qQuestProgress = `query LoorekQuestProgress { currentUser { inventory { viewerRewardDropCampaignsInProgress { id ` +
		`rewardGroups { id name self { status currentMinutesWatched currentSubs grantCount nextEligibleAt currentWindow { expiresAt } } ` +
		`rewards { id dropDetail { badge { setID version title imageURL } } } } } completedRewardCampaigns { id } } } }`
)

type questProgress struct {
	campID, name, status string
	cur, grants          int64
	next, window         time.Time
	has                  bool
	img                  string
}

// fetchQuests returns the reward campaigns as campaigns. ok=false: Twitch did not answer, keep the old ones.
func (e *Engine) fetchQuests(now time.Time, uid string) (out []*Campaign, ok bool) {
	avail, err := e.c.gqlQuery(qQuests, nil)
	if err != nil {
		e.log.Debug("reward campaigns: %v", err)
		return nil, false
	}
	prog, err := e.c.gqlQuery(qQuestProgress, nil)
	if err != nil {
		e.log.Debug("reward campaign progress: %v", err)
		prog = nil // the campaigns are still worth showing
	}
	e.dumpQuests(avail, prog)

	inv := dig(prog, "currentUser", "inventory")
	done := map[string]bool{}
	for _, c := range list(dig(inv, "completedRewardCampaigns")) {
		done[str(dig(c, "id"))] = true
	}
	progress := map[string]questProgress{}
	for _, c := range list(dig(inv, "viewerRewardDropCampaignsInProgress")) {
		for _, g := range list(dig(c, "rewardGroups")) {
			self := dig(g, "self")
			p := questProgress{campID: str(dig(c, "id")), name: str(dig(g, "name")), has: self != nil,
				status: str(dig(self, "status")), cur: num(dig(self, "currentMinutesWatched")), grants: num(dig(self, "grantCount")),
				next: parseTime(dig(self, "nextEligibleAt")), window: parseTime(dig(self, "currentWindow", "expiresAt"))}
			for _, r := range list(dig(g, "rewards")) {
				if p.img == "" {
					p.img = str(dig(r, "dropDetail", "badge", "imageURL"))
				}
			}
			progress[str(dig(g, "id"))] = p
		}
	}

	for _, q := range list(dig(avail, "rewardCampaignsAvailableToUser")) {
		id := str(dig(q, "id"))
		if id == "" || done[id] {
			continue
		}
		c := &Campaign{ID: id, Name: str(dig(q, "name")), Status: "ACTIVE", Quest: true, Linked: true,
			Start: parseTime(dig(q, "startsAt")), End: parseTime(dig(q, "endsAt")),
			LinkURL: firstNonEmpty(str(dig(q, "aboutURL")), str(dig(q, "externalURL")))}
		if c.End.IsZero() {
			continue
		}
		finished, started := false, false
		var cats []GameRef
		days := summaryDays(str(dig(q, "summary"))) // "... one hour each day on six different days ..."
		dailyGid := ""                              // the group the description's days are about: the one being watched, else the first one
		if days > 0 {
			for _, g := range list(dig(q, "rewardGroups")) {
				if gid := str(dig(g, "id")); gid != "" && num(dig(g, "unlockRequirements", "minuteWatchedGoal")) > 0 {
					if progress[gid].has {
						dailyGid = gid
						break
					}
					if dailyGid == "" {
						dailyGid = gid
					}
				}
			}
		}
		for _, g := range list(dig(q, "rewardGroups")) {
			gid := str(dig(g, "id"))
			p := progress[gid]
			if p.campID != "" && done[p.campID] {
				finished = true
			}
			req := num(dig(g, "unlockRequirements", "minuteWatchedGoal"))
			if gid == "" || req <= 0 {
				continue // a group earned with subscriptions cannot be farmed by watching
			}
			for _, cat := range list(dig(g, "categories")) {
				gr := gameRef(cat)
				known := gr.ID == ""
				for _, have := range cats {
					if have.ID == gr.ID {
						known = true
					}
				}
				if !known {
					cats = append(cats, gr)
				}
			}
			started = started || p.has
			d := &Drop{ID: gid, Name: p.name, Req: req, Cur: p.cur, HasSelf: p.has, Img: p.img, Later: !p.has}
			var names []string
			for _, r := range list(dig(g, "rewards")) {
				if n := str(dig(r, "name")); n != "" && len(names) < 3 {
					names = append(names, n)
				}
				if d.Img == "" {
					d.Img = str(dig(r, "thumbnailImage", "image1xURL"))
				}
			}
			if d.Name == "" {
				d.Name = strings.Join(names, ", ")
			}
			reopens := p.next // when the next round of a repeating (daily) group opens
			if reopens.IsZero() {
				reopens = p.window
			}
			if daily := p.grants > 0 || !reopens.IsZero() || gid == dailyGid; daily {
				d.Day = int(p.grants) + 1 // a group that repeats: which day (round) this is
				d.Done, d.Days = int(p.grants), days
				if d.Days > 0 && d.Done >= d.Days {
					d.Claimed, d.Cur, d.Day = true, req, d.Days // every day collected
					c.Drops = append(c.Drops, d)
					continue
				}
				if p.cur < req && !p.window.IsZero() && now.Before(p.window) {
					d.Until = p.window // today's round has to be finished by then
				}
			}
			switch {
			case p.status == "CLAIMABLE":
				d.Cur = req
				d.ClaimID = uid + "#" + firstNonEmpty(p.campID, id) + "#" + gid
			case p.status == "CLAIMED" || p.status == "FULFILLED" || (p.grants > 0 && p.cur >= req):
				// Twitch usually grants these rewards by itself and keeps saying IN_PROGRESS with a full
				// counter until the day's window ends: a grant on record plus a full counter is a finished round.
				switch {
				case reopens.IsZero(): // earned for good
					d.Claimed, d.Cur = true, req
				case now.Before(reopens): // today's round is done, the next one opens later
					d.Claimed, d.Cur, d.Next = true, req, reopens
					if d.Day > 1 {
						d.Day--
					}
				default: // a new round has opened
					d.Cur, d.HasSelf = 0, false
				}
			}
			if d.Cur > req {
				d.Cur = req
			}
			c.Drops = append(c.Drops, d)
		}
		switch {
		case len(cats) == 1:
			c.Game = cats[0]
		case len(cats) > 1:
			// It counts in many categories, so it belongs to none of them: it is listed under its own
			// name, and any of the categories will do for watching.
			c.Game = GameRef{ID: "quest-" + id, Name: c.Name, Art: cats[0].Art}
			c.Cats = cats
		default:
			c.Game = gameRef(dig(q, "game"))
		}
		if !started { // nothing begun yet: every stage is open. Once some are, Twitch counts only those; the rest open later
			for _, d := range c.Drops {
				d.Later = false
			}
		}
		if finished || c.Game.ID == "" || len(c.Drops) == 0 {
			continue // nothing to earn by watching, or no category to watch it in
		}
		if c.Game.Art == "" {
			c.Game.Art = "https://static-cdn.jtvnw.net/ttv-boxart/" + c.Game.ID + "-144x192.jpg"
		}
		out = append(out, c)
	}
	return out, true
}

// dumpQuests keeps Twitch's last answers about reward campaigns next to the journal. These campaigns are
// new and change shape; when one is shown wrongly, this file says what Twitch actually sent. It holds
// campaign and reward names and the account's progress, nothing that signs in.
func (e *Engine) dumpQuests(avail, prog map[string]any) {
	raw, err := json.MarshalIndent(map[string]any{"at": time.Now().Format(time.RFC3339), "available": avail, "progress": prog}, "", " ")
	if err != nil || len(raw) > 2<<20 {
		return
	}
	os.WriteFile(filepath.Join(e.dir, "quests-debug.json"), raw, 0o600)
}

// summaryDays reads how many days a daily reward runs from the campaign's description, which is the only
// place Twitch says it ("... watching for one hour each day on six different days ..."). 0 = not said.
func summaryDays(s string) int {
	s = strings.ToLower(s)
	m := reDays.FindStringSubmatch(s)
	if m == nil || (m[2] == "" && !reDaily.MatchString(s)) {
		return 0 // "opened within 30 days" is not a daily reward: it must be "on six different days", or said to be daily
	}
	if n, err := strconv.Atoi(m[1]); err == nil {
		return n
	}
	for i, w := range []string{"one", "two", "three", "four", "five", "six", "seven", "eight", "nine", "ten", "eleven", "twelve", "thirteen", "fourteen"} {
		if w == m[1] {
			return i + 1
		}
	}
	return 0
}

var reDays = regexp.MustCompile(`\b(\d{1,2}|one|two|three|four|five|six|seven|eight|nine|ten|eleven|twelve|thirteen|fourteen)\s+(different\s+|separate\s+|unique\s+)?days\b`)

var reDaily = regexp.MustCompile(`\b(each|every|per)\s+day\b|\bdaily\b`)
