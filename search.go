package main

import (
	"errors"
	"net/url"
	"strings"
	"sync"
	"time"
)

// Suggestions shown while the user types a channel or a game: the panel asks, this file answers
// from Twitch's and Kick's own searches. Answers are kept for a minute, so typing does not flood them.

const (
	qSearchUsers = `query($q:String!){searchUsers(userQuery:$q,first:8){edges{node{id login displayName profileImageURL(width:70) stream{id} followers{totalCount}}}}}`
	qSearchGames = `query($q:String!){searchCategories(query:$q,first:8){edges{node{id name displayName slug boxArtURL(width:144,height:192)}}}}`
)

type searchHit struct {
	at    time.Time
	items []map[string]any
}

var searchMu sync.Mutex
var searchCache = map[string]searchHit{}

func (e *Engine) search(kind, q string) ([]map[string]any, error) {
	q = strings.TrimSpace(q)
	if r := []rune(q); len(r) > 50 {
		q = string(r[:50])
	}
	none := []map[string]any{}
	if len([]rune(q)) < 2 {
		return none, nil
	}
	key := kind + "\x00" + strings.ToLower(q)
	searchMu.Lock()
	hit, ok := searchCache[key]
	searchMu.Unlock()
	if ok && time.Since(hit.at) < time.Minute {
		return hit.items, nil
	}
	var items []map[string]any
	var err error
	switch kind {
	case "twitch":
		items, err = e.searchTwitch(q)
	case "game":
		items, err = e.searchGames(q)
	case "kick":
		items, err = e.searchKick(q)
	default:
		return none, errors.New("search: unknown kind")
	}
	if err != nil {
		if errors.Is(err, errAuth) {
			e.authLost()
		}
		e.log.Debug("search %s: %v", kind, err)
		return none, err
	}
	if items == nil {
		items = none
	}
	searchMu.Lock()
	if len(searchCache) > 300 {
		searchCache = map[string]searchHit{}
	}
	searchCache[key] = searchHit{time.Now(), items}
	searchMu.Unlock()
	return items, nil
}

func (e *Engine) searchTwitch(q string) ([]map[string]any, error) {
	data, err := e.c.gqlQuery(qSearchUsers, map[string]any{"q": q})
	if err != nil {
		return nil, err
	}
	var out []map[string]any
	for _, ed := range list(dig(data, "searchUsers", "edges")) {
		n := dig(ed, "node")
		login := str(dig(n, "login"))
		if login == "" {
			continue
		}
		out = append(out, map[string]any{"login": login, "name": firstNonEmpty(str(dig(n, "displayName")), login), "img": str(dig(n, "profileImageURL")),
			"live": dig(n, "stream") != nil, "followers": num(dig(n, "followers", "totalCount"))})
	}
	return out, nil
}

func (e *Engine) searchGames(q string) ([]map[string]any, error) {
	data, err := e.c.gqlQuery(qSearchGames, map[string]any{"q": q})
	if err != nil {
		return nil, err
	}
	var out []map[string]any
	for _, ed := range list(dig(data, "searchCategories", "edges")) {
		g := gameRef(dig(ed, "node"))
		if g.ID == "" {
			continue
		}
		out = append(out, map[string]any{"id": g.ID, "slug": g.Slug, "name": g.Name, "art": g.Art})
	}
	return out, nil
}

func (e *Engine) searchKick(q string) ([]map[string]any, error) {
	e.mu.Lock()
	token := e.auth.KickToken
	e.mu.Unlock()
	base := strings.TrimRight(e.c.ep.Kick, "/")
	// never let a stuck Kick connection hold the panel's request for longer than a few seconds
	type answer struct {
		st  int
		v   any
		err error
	}
	done := make(chan answer, 1)
	go func() {
		st, v, err := e.kick.get(base+"/api/search?searched_word="+url.QueryEscape(q), nil, token)
		done <- answer{st, v, err}
	}()
	var st int
	var v any
	select {
	case a := <-done:
		if a.err != nil {
			return nil, a.err
		}
		st, v = a.st, a.v
	case <-time.After(10 * time.Second):
		return nil, errors.New("kick search: no answer")
	}
	if st != 200 {
		return nil, errors.New("kick search: unexpected answer")
	}
	var out []map[string]any
	for _, c := range list(dig(v, "channels")) {
		slug := str(dig(c, "slug"))
		if slug == "" || boolean(dig(c, "is_banned")) {
			continue
		}
		followers := num(dig(c, "followers_count"))
		if followers == 0 {
			followers = num(dig(c, "followersCount"))
		}
		out = append(out, map[string]any{"login": slug, "name": firstNonEmpty(str(dig(c, "user", "username")), slug),
			"img":  firstNonEmpty(str(dig(c, "user", "profilePic")), str(dig(c, "user", "profile_pic"))),
			"live": boolean(dig(c, "isLive")) || boolean(dig(c, "is_live")), "followers": followers})
		if len(out) >= 8 {
			break
		}
	}
	return out, nil
}
