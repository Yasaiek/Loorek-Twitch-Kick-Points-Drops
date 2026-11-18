// Mock of the parts of Twitch that Dropomat talks to. Development and tests only.
package main

import (
	"bufio"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"sort"
	"strings"
	"sync"
	"time"

	"golang.org/x/net/websocket"
)

type channel struct {
	ID, Login, Name string
	Live            bool
	GameID          string
	Viewers         int
	Balance         int
	Claim           string
	Touches         int
	LastTouch       time.Time
	Spade           int
	Drops           []string // campaign ids available on this channel
}

type drop struct {
	ID, Name string
	Req      int
	Pre      []string
	Benefit  string
	Cur      float64
	Claimed  bool
	Touched  bool
}

type campaign struct {
	ID, Name, GameID string
	Linked           bool
	ACL              []string
	Drops            []*drop
}

type game struct{ ID, Name, Slug string }

type kch struct {
	ID, UserID, StreamID int64
	Slug, Name           string
	Live                 bool
	Points, Events       int
}

var kshakes = 0
var kchans = map[string]*kch{
	"kalfa":  {ID: 501, UserID: 601, StreamID: 9001, Slug: "kalfa", Name: "KAlfa", Live: true, Points: 200},
	"kbeta":  {ID: 502, UserID: 602, StreamID: 9002, Slug: "kbeta", Name: "KBeta", Live: true, Points: 50},
	"kgamma": {ID: 503, UserID: 603, Slug: "kgamma", Name: "KGamma", Live: false},
	"kdelta": {ID: 504, UserID: 604, StreamID: 9004, Slug: "kdelta", Name: "KDelta", Live: true},
}

var (
	mu       sync.Mutex
	base     string
	games    = map[string]*game{"g1": {"g1", "Space Game", "space-game"}, "g2": {"g2", "Farm Story", "farm-story"}, "g3": {"g3", "Unlinked Quest", "unlinked-quest"}, "g4": {"g4", "Neon Coast", "neon-coast"}}
	channels = map[string]*channel{}
	camps    = []*campaign{
		{ID: "c1", Name: "Space Drops", GameID: "g1", Linked: true, Drops: []*drop{{ID: "d1", Name: "Helmet", Req: 2, Benefit: "b1"}, {ID: "d2", Name: "Suit", Req: 3, Pre: []string{"d1"}, Benefit: "b2"}}},
		{ID: "c2", Name: "Quest Drops", GameID: "g3", Linked: false, Drops: []*drop{{ID: "d9", Name: "Sword", Req: 60, Benefit: "b9"}}},
		{ID: "c3", Name: "Farm Drops", GameID: "g2", Linked: true, ACL: []string{"beta"}, Drops: []*drop{{ID: "d3", Name: "Tractor", Req: 2, Benefit: "b3"}}},
	}
	// One of Twitch's own reward campaigns: a daily chest on game g4. Watching a g4 stream for questReq
	// minutes makes it claimable; after the claim the next "day" opens questGap later, questDays times in all.
	quest = struct {
		Cur    float64
		Grants int
		Status string // "", IN_PROGRESS, CLAIMABLE, CLAIMED
		Next   time.Time
	}{}
	awarded    = map[string]time.Time{}
	joins      = map[string]bool{}
	bonusEvery = 4
	questReq   = 2
	questDays  = 3
	questGap   = 12 * time.Second
	perTouch   = 0.25 // minutes of drop progress per playlist read
	claimsPts  = 0
	claimsDrop = 0
	// MOCK_QUEST_MODE: "" = the reward must be claimed; "auto" = Twitch grants it by itself and keeps saying
	// IN_PROGRESS with a full counter until the day's window ends; "stuck" = watching never moves the group
	// the account has, while the watching session reports another, not started group with ever more minutes.
	questMode   = os.Getenv("MOCK_QUEST_MODE")
	questClaims = 0 // claim requests for the reward campaign
	dashCount   = 0 // full campaign list readings
	gqlCount    = 0
	headCount   = 0
)

func addCh(login, gameID string, live bool, viewers int, drops ...string) {
	channels[login] = &channel{ID: "id-" + login, Login: login, Name: strings.ToUpper(login[:1]) + login[1:], Live: live, GameID: gameID, Viewers: viewers, Balance: 1000, Drops: drops}
}

// demoReal swaps the test data for a showcase set: real game titles with their covers (files in MOCK_ART_DIR)
// and real channel names. Used only for screenshots.
func demoReal() {
	games = map[string]*game{
		"g0": {"g0", "Just Chatting", "just-chatting"}, "g23": {"g23", "Cyberpunk 2077", "cyberpunk-2077"}, "g27": {"g27", "Sea of Thieves", "sea-of-thieves"},
		"g26": {"g26", "Rainbow Six Siege", "rainbow-six-siege"}, "g24": {"g24", "Delta Force", "delta-force"}, "g21": {"g21", "Apex Legends", "apex-legends"},
		"g25": {"g25", "Genshin Impact", "genshin-impact"}, "g22": {"g22", "Crimson Desert", "crimson-desert"},
	}
	many := func(prefix string, n int, req func(i int) int) []*drop {
		var out []*drop
		for i := 1; i <= n; i++ {
			id := fmt.Sprintf("%s%d", prefix, i)
			out = append(out, &drop{ID: id, Name: fmt.Sprintf("Reward %d", i), Req: req(i), Benefit: id})
		}
		return out
	}
	sot := many("sot", 7, func(i int) int {
		if i < 3 {
			return 30
		}
		return 60
	})
	sot[2].Name = "Face, Meet Palm Emote"
	for _, d := range sot[:2] {
		d.Cur, d.Claimed, d.Touched = float64(d.Req), true, true
		awarded[d.Benefit] = time.Now().Add(-time.Hour)
	}
	sot[2].Cur, sot[2].Touched = 12, true
	camps = []*campaign{
		{ID: "R3", Name: "Quadra Paintjobs", GameID: "g23", Linked: true, Drops: []*drop{{ID: "cp1", Name: "Quadra Paintjobs", Req: 120, Benefit: "cp1", Cur: 47, Touched: true}}},
		{ID: "R8", Name: "Twitch Drops", GameID: "g27", Linked: true, Drops: sot},
		{ID: "R7", Name: "Twitch Drops", GameID: "g26", Linked: true, Drops: many("r6", 6, func(int) int { return 60 })},
		{ID: "R4", Name: "Delta Force Drops", GameID: "g24", Linked: false, Drops: many("df", 2, func(int) int { return 60 })},
		{ID: "R1", Name: "Apex Legends | Godzilla", GameID: "g21", Linked: true, Drops: many("ax", 1, func(int) int { return 60 })},
		{ID: "R6", Name: "V7.2 Genshin Impact Drops", GameID: "g25", Linked: true, Drops: many("gi", 1, func(int) int { return 60 })},
		{ID: "R2", Name: "Crimson Desert DLC Week 2", GameID: "g22", Linked: true, Drops: many("cd", 1, func(int) int { return 60 })},
	}
	if hide := os.Getenv("MOCK_DEMO_HIDE"); hide != "" { // a game that is known but has no campaign (for the tracked-games shot)
		kept := camps[:0]
		for _, c := range camps {
			if c.GameID != hide {
				kept = append(kept, c)
			}
		}
		camps = kept
	}
	channels = map[string]*channel{}
	add := func(login, name, gameID string, live bool, viewers, balance int, drops ...string) {
		addCh(login, gameID, live, viewers, drops...)
		channels[login].Name, channels[login].Balance = name, balance
	}
	add("cdprojektred", "CDPROJEKTRED", "g23", true, 4800, 0, "R3")
	add("seaofthieves", "SeaOfThieves", "g27", true, 1900, 0, "R8")
	add("rainbow6", "Rainbow6", "g26", true, 6400, 0, "R7")
	add("h2p_gucio", "H2P_Gucio", "g0", true, 6200, 48210)
	add("xayoo_", "xayoo_", "g0", true, 9100, 12750)
	add("izakooo", "IzakOOO", "g0", false, 0, 8340)
	add("ewroon", "EWROON", "g0", true, 3900, 27600)
	add("pago3", "PAGO3", "g0", false, 0, 1920)
	kchans = map[string]*kch{
		"delordione": {ID: 501, UserID: 601, StreamID: 9001, Slug: "delordione", Name: "delordione", Live: true, Points: 3480},
		"azdus":      {ID: 502, UserID: 602, StreamID: 9002, Slug: "azdus", Name: "Azdus", Live: true, Points: 1210},
		"demonzz1":   {ID: 503, UserID: 603, Slug: "demonzz1", Name: "demonzz1", Live: false, Points: 640},
		"pajalock":   {ID: 504, UserID: 604, StreamID: 9004, Slug: "pajalock", Name: "pajalock", Live: true, Points: 90},
	}
}

func chByID(id string) *channel {
	for _, c := range channels {
		if c.ID == id {
			return c
		}
	}
	return nil
}

func camp(id string) *campaign {
	for _, c := range camps {
		if c.ID == id {
			return c
		}
	}
	return nil
}

// currentDrop: first unfinished drop of a campaign whose preconditions are claimed.
func currentDrop(c *campaign) *drop {
	for _, d := range c.Drops {
		if d.Claimed || d.Cur >= float64(d.Req) {
			continue
		}
		ok := true
		for _, p := range d.Pre {
			for _, o := range c.Drops {
				if o.ID == p && !o.Claimed {
					ok = false
				}
			}
		}
		if ok {
			return d
		}
	}
	return nil
}

func ts(t time.Time) string { return t.UTC().Format(time.RFC3339) }

func gameJSON(id string) map[string]any {
	g := games[id]
	return map[string]any{"id": g.ID, "name": g.Name, "displayName": g.Name, "slug": g.Slug, "boxArtURL": base + "/art/" + g.ID + "-{width}x{height}.jpg"}
}

func dropJSON(c *campaign, d *drop, withSelf bool) map[string]any {
	pre := []any{}
	for _, p := range d.Pre {
		pre = append(pre, map[string]any{"id": p})
	}
	out := map[string]any{
		"id": d.ID, "name": d.Name, "requiredMinutesWatched": d.Req, "preconditionDrops": pre,
		"startAt": ts(time.Now().Add(-24 * time.Hour)), "endAt": ts(time.Now().Add(72 * time.Hour)),
		"benefitEdges": []any{map[string]any{"benefit": map[string]any{"id": d.Benefit, "name": d.Name, "imageAssetURL": base + "/art/" + d.Benefit + ".png"}}},
	}
	if withSelf {
		out["self"] = map[string]any{"currentMinutesWatched": int(d.Cur), "isClaimed": d.Claimed, "dropInstanceID": "1001#" + c.ID + "#" + d.ID, "hasPreconditionsMet": true}
	}
	return out
}

func campJSON(c *campaign, detail, withSelf bool) map[string]any {
	gm := gameJSON(c.GameID)
	if detail || withSelf { // like the real thing: only the campaign list carries the cover
		delete(gm, "boxArtURL")
	}
	out := map[string]any{"id": c.ID, "name": c.Name, "status": "ACTIVE", "game": gm,
		"startAt": ts(time.Now().Add(-24 * time.Hour)), "endAt": ts(time.Now().Add(72 * time.Hour)),
		"accountLinkURL": "https://example.com/link/" + c.ID, "self": map[string]any{"isAccountConnected": c.Linked}}
	if detail || withSelf {
		var drops []any
		for _, d := range c.Drops {
			drops = append(drops, dropJSON(c, d, withSelf))
		}
		out["timeBasedDrops"] = drops
	}
	if detail {
		var acl any
		if len(c.ACL) > 0 {
			var l []any
			for _, a := range c.ACL {
				ch := channels[a]
				l = append(l, map[string]any{"id": ch.ID, "name": ch.Login, "displayName": ch.Name})
			}
			acl = l
		}
		out["allow"] = map[string]any{"channels": acl, "isEnabled": true}
	}
	return out
}

func handleOp(name string, v map[string]any) (any, string) {
	s := func(k string) string { x, _ := v[k].(string); return x }
	switch name {
	case "VideoPlayerStreamInfoOverlayChannel":
		ch := channels[s("channel")]
		if ch == nil {
			return map[string]any{"user": nil}, ""
		}
		var stream any
		if ch.Live {
			stream = map[string]any{"id": "st-" + ch.Login, "viewersCount": ch.Viewers}
		}
		return map[string]any{"user": map[string]any{"id": ch.ID, "displayName": ch.Name, "stream": stream, "profileImageURL": base + "/art/user-" + ch.Login + ".png",
			"broadcastSettings": map[string]any{"title": "t", "game": gameJSON(ch.GameID)}}}, ""
	case "ChannelPointsContext":
		ch := channels[s("channelLogin")]
		if ch == nil {
			return map[string]any{"community": nil}, ""
		}
		var claim any
		if ch.Claim != "" {
			claim = map[string]any{"id": ch.Claim}
		}
		return map[string]any{"community": map[string]any{"id": ch.ID, "channel": map[string]any{"self": map[string]any{
			"communityPoints": map[string]any{"balance": ch.Balance, "availableClaim": claim}}}}}, ""
	case "ClaimCommunityPoints":
		in, _ := v["input"].(map[string]any)
		ch := chByID(fmt.Sprint(in["channelID"]))
		if ch == nil || ch.Claim == "" || ch.Claim != fmt.Sprint(in["claimID"]) {
			return nil, "claim not found"
		}
		ch.Claim = ""
		ch.Balance += 50
		claimsPts++
		return map[string]any{"claimCommunityPoints": map[string]any{"claim": map[string]any{"id": in["claimID"]}}}, ""
	case "PlaybackAccessToken":
		return map[string]any{"streamPlaybackAccessToken": map[string]any{"value": `{"channel":"` + s("login") + `"}`, "signature": "sig"}}, ""
	case "Inventory":
		var prog []any
		for _, c := range camps {
			touched := false
			for _, d := range c.Drops {
				if d.Touched {
					touched = true
				}
			}
			allClaimed := true
			for _, d := range c.Drops {
				if !d.Claimed {
					allClaimed = false
				}
			}
			if touched && !allClaimed { // a fully claimed campaign leaves the in-progress list
				prog = append(prog, campJSON(c, false, true))
			}
		}
		var ev []any
		for b, at := range awarded {
			ev = append(ev, map[string]any{"id": b, "lastAwardedAt": ts(at)})
		}
		return map[string]any{"currentUser": map[string]any{"inventory": map[string]any{"dropCampaignsInProgress": prog, "gameEventDrops": ev}}}, ""
	case "ViewerDropsDashboard":
		dashCount++
		var l []any
		for _, c := range camps {
			l = append(l, campJSON(c, false, false))
		}
		return map[string]any{"currentUser": map[string]any{"dropCampaigns": l}}, ""
	case "DropCampaignDetails":
		c := camp(s("dropID"))
		if c == nil {
			return map[string]any{"user": map[string]any{"dropCampaign": nil}}, ""
		}
		return map[string]any{"user": map[string]any{"dropCampaign": campJSON(c, true, false)}}, ""
	case "DropCurrentSessionContext":
		ch := chByID(s("channelID"))
		if ch == nil || time.Since(ch.LastTouch) > 5*time.Second {
			return map[string]any{"currentUser": map[string]any{"dropCurrentSession": nil}}, ""
		}
		if ch.GameID == "g4" && questMode == "stuck" { // minutes pile up on a group the account has not started
			return map[string]any{"currentUser": map[string]any{"dropCurrentSession": map[string]any{
				"dropID": "QG0", "currentMinutesWatched": 250 + ch.Touches/4, "requiredMinutesWatched": questReq}}}, ""
		}
		for _, id := range ch.Drops {
			if c := camp(id); c != nil && c.Linked {
				if d := currentDrop(c); d != nil {
					return map[string]any{"currentUser": map[string]any{"dropCurrentSession": map[string]any{
						"dropID": d.ID, "currentMinutesWatched": int(d.Cur), "requiredMinutesWatched": d.Req}}}, ""
				}
			}
		}
		return map[string]any{"currentUser": map[string]any{"dropCurrentSession": nil}}, ""
	case "DropsHighlightService_AvailableDrops":
		ch := chByID(s("channelID"))
		var l []any
		if ch != nil {
			for _, id := range ch.Drops {
				l = append(l, map[string]any{"id": id})
			}
		}
		return map[string]any{"channel": map[string]any{"viewerDropCampaigns": l}}, ""
	case "DirectoryPage_Game":
		var edges []any
		for _, ch := range channels {
			if ch.Live && games[ch.GameID].Slug == s("slug") && len(ch.Drops) > 0 {
				edges = append(edges, map[string]any{"node": map[string]any{"id": "st-" + ch.Login, "viewersCount": ch.Viewers, "title": "t",
					"broadcaster": map[string]any{"id": ch.ID, "login": ch.Login, "displayName": ch.Name}, "game": gameJSON(ch.GameID)}})
			}
		}
		return map[string]any{"game": map[string]any{"streams": map[string]any{"edges": edges}}}, ""
	case "DirectoryGameRedirect":
		for _, g := range games {
			if g.Name == s("name") {
				return map[string]any{"game": map[string]any{"slug": g.Slug}}, ""
			}
		}
		return map[string]any{"game": nil}, ""
	case "DropsPage_ClaimDropRewards":
		in, _ := v["input"].(map[string]any)
		parts := strings.Split(fmt.Sprint(in["dropInstanceID"]), "#")
		if len(parts) == 3 && parts[1] == "Q1" {
			questClaims++
			if questMode != "" { // nothing to claim there, yet Twitch answers as if all were well
				return map[string]any{"claimDropRewards": map[string]any{"status": "ELIGIBLE_FOR_ALL"}}, ""
			}
		}
		if len(parts) == 3 && parts[1] == "Q1" && parts[2] == "QG1" && quest.Status == "CLAIMABLE" {
			quest.Status, quest.Next = "CLAIMED", time.Now().Add(questGap)
			quest.Grants++
			claimsDrop++
			return map[string]any{"claimDropRewards": map[string]any{"status": "ELIGIBLE_FOR_ALL"}}, ""
		}
		if len(parts) == 3 {
			if c := camp(parts[1]); c != nil {
				for _, d := range c.Drops {
					if d.ID == parts[2] && d.Cur >= float64(d.Req) {
						st := "ELIGIBLE_FOR_ALL"
						if d.Claimed {
							st = "DROP_INSTANCE_ALREADY_CLAIMED"
						}
						d.Claimed = true
						awarded[d.Benefit] = time.Now()
						claimsDrop++
						return map[string]any{"claimDropRewards": map[string]any{"status": st}}, ""
					}
				}
			}
		}
		return map[string]any{"claimDropRewards": nil}, ""
	}
	return nil, "PersistedQueryNotFound"
}

func main() {
	port := "8799"
	if len(os.Args) > 1 {
		port = os.Args[1]
	}
	base = "http://127.0.0.1:" + port
	addCh("alfa", "g1", true, 500)
	addCh("beta", "g2", true, 300, "c3")
	addCh("gamma", "g2", false, 0)
	addCh("delta", "g1", true, 900, "c1")
	addCh("epsilon", "g1", true, 200, "c1")
	addCh("zeta", "g4", true, 700, "quest") // drops are on, but its only campaign is the reward campaign
	if os.Getenv("MOCK_DEMO") == "real" {
		demoReal()
	}
	if n, err := time.ParseDuration(os.Getenv("MOCK_QUEST_GAP")); err == nil && n > 0 {
		questGap = n // how long the reward campaign's next "day" takes to open
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/oauth2/validate", func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "OAuth goodtoken" {
			w.WriteHeader(401)
			fmt.Fprint(w, `{"status":401,"message":"invalid access token"}`)
			return
		}
		fmt.Fprint(w, `{"client_id":"kd1unb4b3q4t58fwlpcbzcbnm76a8fp","login":"viewer","user_id":"1001","expires_in":5000000}`)
	})
	mux.HandleFunc("/gql", func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "OAuth goodtoken" {
			w.WriteHeader(401)
			fmt.Fprint(w, `{"error":"Unauthorized","status":401,"message":"The \"Authorization\" token is invalid."}`)
			return
		}
		raw, _ := io.ReadAll(r.Body)
		var ops []map[string]any
		if json.Unmarshal(raw, &ops) != nil {
			var one map[string]any
			json.Unmarshal(raw, &one)
			ops = []map[string]any{one}
		}
		mu.Lock()
		defer mu.Unlock()
		gqlCount++
		var out []any
		if q, _ := ops[0]["query"].(string); q != "" && len(ops) == 1 && (strings.Contains(q, "rewardCampaignsAvailableToUser") || strings.Contains(q, "viewerRewardDropCampaignsInProgress")) {
			// Twitch's own reward campaigns
			w.Header().Set("Content-Type", "application/json")
			demo := os.Getenv("MOCK_DEMO") == "real" // (the handler already holds the lock)
			if strings.Contains(q, "rewardCampaignsAvailableToUser") {
				list := []any{}
				if !demo {
					cat := map[string]any{"id": "g4", "displayName": "Neon Coast", "slug": "neon-coast", "boxArtURL": base + "/art/g4-{width}x{height}.jpg"}
					cats := []any{cat}
					if os.Getenv("MOCK_QUEST_CATS") == "multi" { // counts in several categories; nobody streams the first one
						cats = []any{map[string]any{"id": "g9", "displayName": "Empty Bay", "slug": "empty-bay", "boxArtURL": base + "/art/g9-{width}x{height}.jpg"}, cat}
					}
					reward := func(id, name string) map[string]any {
						return map[string]any{"id": id, "name": name, "thumbnailImage": map[string]any{"image1xURL": base + "/art/" + id + ".png"}}
					}
					list = append(list, map[string]any{"id": "Q1", "name": "Neon Launch Celebration", "brand": "Twitch", "isSitewide": true, "game": nil,
						"startsAt": ts(time.Now().Add(-time.Hour)), "endsAt": ts(time.Now().Add(48 * time.Hour)), "aboutURL": base + "/about-quest",
						"rewardGroups": []any{
							map[string]any{"id": "QG0", "unlockRequirements": map[string]any{"minuteWatchedGoal": questReq, "subsGoal": 0, "turboSubsGoal": 0, "eventTriggerGoal": 0},
								"categories": cats, "rewards": []any{reward("qg0", "Ghost Stage")}},
							map[string]any{"id": "QG1", "unlockRequirements": map[string]any{"minuteWatchedGoal": questReq, "subsGoal": 0, "turboSubsGoal": 0, "eventTriggerGoal": 0},
								"categories": cats, "rewards": []any{reward("qb1", "Purple Chain"), reward("qb2", "Bronze Chain"), reward("qb3", "Silver Chain")}},
							map[string]any{"id": "QG2", "unlockRequirements": map[string]any{"minuteWatchedGoal": 0, "subsGoal": 3, "turboSubsGoal": 0, "eventTriggerGoal": 0},
								"categories": []any{cat}, "rewards": []any{reward("qe1", "Sub Emote")}},
						}})
				}
				json.NewEncoder(w).Encode(map[string]any{"data": map[string]any{"rewardCampaignsAvailableToUser": list}})
				return
			}
			inProgress, completed := []any{}, []any{}
			if !demo && questMode != "" && (quest.Status != "" || quest.Grants > 0 || questMode == "stuck") {
				self := map[string]any{"status": "IN_PROGRESS", "currentMinutesWatched": int(quest.Cur), "currentSubs": 0, "grantCount": quest.Grants,
					"nextEligibleAt": nil, "currentWindow": nil}
				if !quest.Next.IsZero() {
					self["currentWindow"] = map[string]any{"expiresAt": ts(quest.Next)}
				}
				inProgress = append(inProgress, map[string]any{"id": "Q1", "rewardGroups": []any{map[string]any{"id": "QG1", "name": "Neon Chest", "self": self, "rewards": []any{}}}})
			} else if !demo && (quest.Status != "" || quest.Grants > 0) {
				self := map[string]any{"status": quest.Status, "currentMinutesWatched": int(quest.Cur), "currentSubs": 0, "grantCount": quest.Grants,
					"nextEligibleAt": nil, "currentWindow": map[string]any{"expiresAt": ts(time.Now().Add(time.Hour))}}
				if quest.Status == "" {
					self["status"] = "CLAIMED" // Twitch keeps saying "claimed" until the new day's watching starts
				}
				if !quest.Next.IsZero() && quest.Grants < questDays {
					self["nextEligibleAt"] = ts(quest.Next)
				}
				inProgress = append(inProgress, map[string]any{"id": "Q1", "rewardGroups": []any{map[string]any{"id": "QG1", "name": "Neon Chest", "self": self, "rewards": []any{}}}})
			}
			if quest.Grants >= questDays {
				completed = append(completed, map[string]any{"id": "Q1"})
			}
			json.NewEncoder(w).Encode(map[string]any{"data": map[string]any{"currentUser": map[string]any{"inventory": map[string]any{
				"viewerRewardDropCampaignsInProgress": inProgress, "completedRewardCampaigns": completed}}}})
			return
		}
		if q, _ := ops[0]["query"].(string); q != "" && len(ops) == 1 { // a plain query: the searches behind the suggestions
			vars, _ := ops[0]["variables"].(map[string]any)
			word := strings.ToLower(fmt.Sprint(vars["q"]))
			edges := []any{}
			if strings.Contains(q, "searchUsers") {
				for _, ch := range channels {
					if strings.Contains(ch.Login, word) {
						var stream any
						if ch.Live {
							stream = map[string]any{"id": "s" + ch.ID}
						}
						edges = append(edges, map[string]any{"node": map[string]any{"id": ch.ID, "login": ch.Login, "displayName": ch.Name,
							"profileImageURL": base + "/art/user-" + ch.Login + ".png", "stream": stream, "followers": map[string]any{"totalCount": 1000 * len(ch.Login)}}})
					}
				}
				w.Header().Set("Content-Type", "application/json")
				json.NewEncoder(w).Encode(map[string]any{"data": map[string]any{"searchUsers": map[string]any{"edges": edges}}})
				return
			}
			all := []game{}
			for _, g := range games {
				all = append(all, *g)
			}
			if os.Getenv("MOCK_DEMO") != "real" {
				all = append(all, game{"g77", "Nowhere Kings", "nowhere-kings"}, game{"g78", "Nowhere Kings II", "nowhere-kings-ii"})
			}
			sort.Slice(all, func(i, j int) bool { return all[i].Name < all[j].Name })
			for _, g := range all {
				if strings.Contains(strings.ToLower(g.Name), word) {
					edges = append(edges, map[string]any{"node": map[string]any{"id": g.ID, "name": g.Name, "displayName": g.Name, "slug": g.Slug, "boxArtURL": base + "/art/" + g.ID + "-144x192.jpg"}})
				}
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]any{"data": map[string]any{"searchCategories": map[string]any{"edges": edges}}})
			return
		}
		for _, op := range ops {
			name, _ := op["operationName"].(string)
			vars, _ := op["variables"].(map[string]any)
			data, errMsg := handleOp(name, vars)
			res := map[string]any{"data": data, "extensions": map[string]any{"operationName": name}}
			if errMsg != "" {
				res["errors"] = []any{map[string]any{"message": errMsg}}
			}
			out = append(out, res)
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(out)
	})
	mux.HandleFunc("/api/channel/hls/", func(w http.ResponseWriter, r *http.Request) {
		login := strings.TrimSuffix(strings.TrimPrefix(r.URL.Path, "/api/channel/hls/"), ".m3u8")
		mu.Lock()
		ch := channels[login]
		live := ch != nil && ch.Live
		mu.Unlock()
		if !live {
			w.WriteHeader(404)
			fmt.Fprint(w, `[{"error":"Can not find channel","error_code":"transcode_does_not_exist"}]`)
			return
		}
		fmt.Fprintf(w, "#EXTM3U\n#EXT-X-STREAM-INF:BANDWIDTH=6000000\n%s/hls/%s/best.m3u8\n#EXT-X-STREAM-INF:BANDWIDTH=230000\n%s/hls/%s/low.m3u8\n", base, login, base, login)
	})
	mux.HandleFunc("/hls/", func(w http.ResponseWriter, r *http.Request) {
		parts := strings.Split(strings.TrimPrefix(r.URL.Path, "/hls/"), "/")
		mu.Lock()
		defer mu.Unlock()
		ch := channels[parts[0]]
		if ch == nil || !ch.Live {
			w.WriteHeader(404)
			return
		}
		if r.Method == "HEAD" {
			headCount++
			return
		}
		ch.Touches++
		ch.LastTouch = time.Now()
		if ch.GameID == "g4" && questMode == "auto" && quest.Grants < questDays {
			if !quest.Next.IsZero() && !time.Now().Before(quest.Next) {
				quest.Cur, quest.Next = 0, time.Time{} // the day's window ended: the counter starts again
			}
			if quest.Next.IsZero() {
				quest.Status = "IN_PROGRESS"
				if quest.Cur += perTouch; quest.Cur >= float64(questReq) {
					quest.Cur, quest.Next = float64(questReq), time.Now().Add(questGap)
					quest.Grants++ // granted without a claim
				}
			}
		} else if ch.GameID == "g4" && questMode == "" && quest.Grants < questDays {
			if quest.Status == "CLAIMED" && !time.Now().Before(quest.Next) {
				quest.Status, quest.Cur = "", 0 // the next day has opened
			}
			if quest.Status == "" || quest.Status == "IN_PROGRESS" {
				quest.Status = "IN_PROGRESS"
				if quest.Cur += perTouch; quest.Cur >= float64(questReq) {
					quest.Cur, quest.Status = float64(questReq), "CLAIMABLE"
				}
			}
		}
		for _, id := range ch.Drops {
			if c := camp(id); c != nil && c.Linked {
				if d := currentDrop(c); d != nil {
					d.Cur += perTouch
					d.Touched = true
					if d.Cur > float64(d.Req) {
						d.Cur = float64(d.Req)
					}
				}
				break
			}
		}
		fmt.Fprint(w, "#EXTM3U\n")
		for i := 0; i < 5; i++ {
			fmt.Fprintf(w, "#EXTINF:2.0,\n%s/hls/%s/chunk%d.ts\n", base, ch.Login, i)
		}
	})
	mux.HandleFunc("/spade", func(w http.ResponseWriter, r *http.Request) {
		r.ParseForm()
		raw, _ := base64.StdEncoding.DecodeString(r.Form.Get("data"))
		var ev []map[string]any
		json.Unmarshal(raw, &ev)
		mu.Lock()
		if len(ev) > 0 {
			props, _ := ev[0]["properties"].(map[string]any)
			// like Twitch: a report that does not say it comes from the web player on a channel page is
			// accepted (204) but does not count
			// Points come from these reports, not from reading the playlist.
			if ch := channels[fmt.Sprint(props["channel"])]; ch != nil && props["player"] == "site" && props["location"] == "channel" {
				ch.Spade++
				ch.Balance += 10
				if ch.Claim == "" && ch.Spade%bonusEvery == 0 {
					ch.Claim = fmt.Sprintf("claim-%s-%d", ch.Login, ch.Spade)
				}
			}
		}
		mu.Unlock()
		w.WriteHeader(204)
	})
	mux.HandleFunc("/login", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		fmt.Fprint(w, `<!doctype html><title>Log In - Twitch (mock)</title><body style="font:16px system-ui;background:#18181b;color:#fff;padding:40px">
<h2>Twitch (mock)</h2><p>Log in to continue.</p><button id=b style="padding:10px 18px">Log In</button>
<script>const AUTO=`+fmt.Sprint(os.Getenv("MOCK_AUTOLOGIN") == "1")+`;const go=()=>{location.href='/#access_token=goodtoken&scope=&token_type=bearer'};document.getElementById('b').onclick=go;if(location.search.includes('auto')||AUTO)setTimeout(go,1500)</script>`)
	})
	// ---- releases (for the updater): files from the directory named by MOCK_UPD_DIR ----
	mux.HandleFunc("/upd/releases/", func(w http.ResponseWriter, r *http.Request) {
		dir := os.Getenv("MOCK_UPD_DIR")
		name := r.URL.Path[strings.LastIndex(r.URL.Path, "/")+1:]
		raw, err := os.ReadFile(dir + "/" + name)
		if dir == "" || err != nil {
			w.WriteHeader(404)
			return
		}
		w.Write(raw)
	})
	// ---- Kick ----
	kauth := func(r *http.Request) bool {
		return r.Header.Get("Authorization") == "Bearer 1234|kicktokenkicktokenkicktoken"
	}
	mux.HandleFunc("/kick", func(w http.ResponseWriter, r *http.Request) { fmt.Fprint(w, "<html>kick mock</html>") })
	// the site as the sign-in window sees it: a login page, then a signed-in page whose own requests carry the key
	kmode := "good" // good: the session cookie is the key; hdr: only the request header is
	mux.HandleFunc("/ctl/kmode", func(w http.ResponseWriter, r *http.Request) { kmode = r.URL.Query().Get("m") })
	mux.HandleFunc("/kick/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		switch r.URL.Path {
		case "/kick/":
			fmt.Fprint(w, `<!doctype html><title>Kick (mock)</title><body style="font:16px system-ui;background:#0b0e0f;color:#fff;padding:40px">
<h2>Kick (mock)</h2><button data-testid="login" style="display:none">hidden twin</button><button id=b data-testid="login" style="padding:10px 18px">Log In</button>
<script>const go=()=>{location.href='/kick/dologin'};document.getElementById('b').onclick=go</script>`)
		case "/kick/dologin":
			val := "1234%7Ckicktokenkicktokenkicktoken"
			if kmode == "hdr" {
				val = "9999%7Cnotthekeynotthekeynotthekey"
			}
			w.Header().Set("Set-Cookie", "session_token="+val+"; Path=/; HttpOnly")
			http.Redirect(w, r, "/kick/home", 302)
		case "/kick/home":
			fmt.Fprint(w, `<!doctype html><title>Kick (mock)</title><body style="font:16px system-ui;background:#0b0e0f;color:#fff;padding:40px"><h2>Signed in</h2>
<script>fetch('/kick/api/v1/user',{headers:{Authorization:'Bearer 1234|kicktokenkicktokenkicktoken'}})</script>`)
		default:
			w.WriteHeader(404)
		}
	})
	mux.HandleFunc("/kick/api/search", func(w http.ResponseWriter, r *http.Request) {
		word := strings.ToLower(r.URL.Query().Get("searched_word"))
		mu.Lock()
		defer mu.Unlock()
		out := []any{}
		for _, ch := range kchans {
			if strings.Contains(ch.Slug, word) {
				out = append(out, map[string]any{"id": ch.ID, "slug": ch.Slug, "isLive": ch.Live, "followersCount": 4200,
					"user": map[string]any{"id": ch.UserID, "username": ch.Name, "profilePic": base + "/art/kuser-" + ch.Slug + ".png"}})
			}
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{"channels": out, "categories": []any{}})
	})
	mux.HandleFunc("/kick/api/v1/user", func(w http.ResponseWriter, r *http.Request) {
		if !kauth(r) {
			w.WriteHeader(401)
			fmt.Fprint(w, `{"message":"Unauthenticated."}`)
			return
		}
		fmt.Fprint(w, `{"id":77,"username":"viewer_k"}`)
	})
	mux.HandleFunc("/kick/api/v2/channels/", func(w http.ResponseWriter, r *http.Request) {
		parts := strings.Split(strings.TrimPrefix(r.URL.Path, "/kick/api/v2/channels/"), "/")
		mu.Lock()
		defer mu.Unlock()
		ch := kchans[parts[0]]
		if ch == nil {
			w.WriteHeader(404)
			return
		}
		if !kauth(r) {
			w.WriteHeader(401)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		if len(parts) > 1 && parts[1] == "points" {
			json.NewEncoder(w).Encode(map[string]any{"data": map[string]any{"points": ch.Points}})
			return
		}
		var ls any
		if ch.Live {
			ls = map[string]any{"id": ch.StreamID, "is_live": true, "categories": []any{map[string]any{"name": "Just Chatting"}}}
		}
		json.NewEncoder(w).Encode(map[string]any{"id": ch.ID, "user_id": ch.UserID, "slug": ch.Slug, "user": map[string]any{"id": ch.UserID, "username": ch.Name, "profile_pic": base + "/art/kuser-" + ch.Slug + ".png"}, "livestream": ls})
	})
	mux.HandleFunc("/kickws/viewer/v1/token", func(w http.ResponseWriter, r *http.Request) {
		if !kauth(r) || r.Header.Get("X-Chatroom") == "" {
			w.WriteHeader(401)
			return
		}
		fmt.Fprintf(w, `{"data":{"token":"wst-%s"}}`, r.Header.Get("X-Chatroom"))
	})
	mux.Handle("/kickws/viewer/v1/connect", websocket.Server{Handler: func(ws *websocket.Conn) {
		if !strings.HasPrefix(ws.Request().URL.Query().Get("token"), "wst-") {
			return
		}
		for {
			var msg string
			if websocket.Message.Receive(ws, &msg) != nil {
				return
			}
			var m struct {
				Type string `json:"type"`
				Data struct {
					Message struct {
						Name         string `json:"name"`
						ChannelID    int64  `json:"channel_id"`
						LivestreamID int64  `json:"livestream_id"`
						ChannelId    int64  `json:"channelId"`
					} `json:"message"`
				} `json:"data"`
			}
			json.Unmarshal([]byte(msg), &m)
			mu.Lock()
			switch m.Type {
			case "ping":
				websocket.Message.Send(ws, `{"type":"pong"}`)
			case "channel_handshake":
				kshakes++
			case "user_event":
				for _, ch := range kchans {
					if ch.ID == m.Data.Message.ChannelID && ch.Live && ch.StreamID == m.Data.Message.LivestreamID && m.Data.Message.Name == "tracking.user.watch.livestream" {
						ch.Events++
						if ch.Events%3 == 0 {
							ch.Points += 10
						}
					}
				}
			}
			mu.Unlock()
		}
	}})
	mux.HandleFunc("/ctl/klive", func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		if ch := kchans[r.URL.Query().Get("ch")]; ch != nil {
			ch.Live = r.URL.Query().Get("on") == "1"
		}
		mu.Unlock()
	})
	mux.HandleFunc("/ctl/state", func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		drops := map[string]any{}
		for _, c := range camps {
			for _, d := range c.Drops {
				drops[d.ID] = map[string]any{"cur": d.Cur, "claimed": d.Claimed}
			}
		}
		js := []string{}
		for j := range joins {
			js = append(js, j)
		}
		json.NewEncoder(w).Encode(map[string]any{"kick": kchans, "kshakes": kshakes, "channels": channels, "drops": drops, "joins": js, "claimsPts": claimsPts, "claimsDrop": claimsDrop, "questClaims": questClaims, "dash": dashCount, "gql": gqlCount, "heads": headCount})
	})
	mux.HandleFunc("/ctl/live", func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		if ch := channels[r.URL.Query().Get("ch")]; ch != nil {
			ch.Live = r.URL.Query().Get("on") == "1"
		}
		mu.Unlock()
	})
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/" {
			http.SetCookie(w, &http.Cookie{Name: "unique_id", Value: "mockdevice0123456789abcdef012345"})
			fmt.Fprint(w, "<html>twitch mock</html>")
			return
		}
		if dir := os.Getenv("MOCK_ART_DIR"); dir != "" && strings.HasPrefix(r.URL.Path, "/art/") {
			name := strings.TrimPrefix(r.URL.Path, "/art/")
			if i := strings.LastIndex(name, "-"); i > 0 && strings.HasSuffix(name, ".jpg") {
				name = name[:i]
			}
			name = strings.TrimSuffix(name, ".png")
			if g := games[name]; g != nil {
				name = g.Slug
			}
			if raw, err := os.ReadFile(dir + "/" + name + ".png"); err == nil && !strings.Contains(name, "/") {
				w.Header().Set("Content-Type", "image/png")
				w.Write(raw)
				return
			}
		}
		if strings.HasPrefix(r.URL.Path, "/art/") {
			w.Header().Set("Content-Type", "image/svg+xml")
			h := 0
			for _, c := range r.URL.Path {
				h = (h*31 + int(c)) % 360
			}
			fmt.Fprintf(w, `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 30 40"><rect width="30" height="40" fill="hsl(%d 60%% 40%%)"/><circle cx="15" cy="20" r="8" fill="hsl(%d 70%% 65%%)"/></svg>`, h, (h+60)%360)
			return
		}
		// channel page with the address the player reports to
		fmt.Fprint(w, `<html><script>window.__twilightSettings={"spade_url":"https://video-edge.mock/track/abc"}</script></html>`)
	})

	// chat (plain TCP on port+1)
	go func() {
		var p int
		fmt.Sscan(port, &p)
		ln, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", p+1))
		if err != nil {
			return
		}
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			go func(c net.Conn) {
				defer c.Close()
				r := bufio.NewReader(c)
				for {
					line, err := r.ReadString('\n')
					if err != nil {
						return
					}
					line = strings.TrimSpace(line)
					switch {
					case strings.HasPrefix(line, "PASS "):
						if line != "PASS oauth:goodtoken" {
							fmt.Fprint(c, ":tmi.twitch.tv NOTICE * :Login authentication failed\r\n")
							return
						}
					case strings.HasPrefix(line, "NICK "):
						fmt.Fprintf(c, ":tmi.twitch.tv 001 %s :Welcome, GLHF!\r\n", strings.TrimPrefix(line, "NICK "))
					case strings.HasPrefix(line, "JOIN #"):
						mu.Lock()
						joins[strings.TrimPrefix(line, "JOIN #")] = true
						mu.Unlock()
					case strings.HasPrefix(line, "PART #"):
						mu.Lock()
						delete(joins, strings.TrimPrefix(line, "PART #"))
						mu.Unlock()
					case strings.HasPrefix(line, "PING"):
						fmt.Fprint(c, "PONG :tmi.twitch.tv\r\n")
					}
				}
			}(conn)
		}
	}()
	fmt.Println("mock twitch on", base)
	http.ListenAndServe("127.0.0.1:"+port, mux)
}
