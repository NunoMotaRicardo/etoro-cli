package etoro

import (
	"context"
	"net/http"
	"strings"
)

// PortfolioPath returns the portfolio (or PnL) endpoint for the client environment.
func (c *Client) PortfolioPath(pnl bool) string {
	switch {
	case c.Env == "real" && pnl:
		return "/trading/info/real/pnl"
	case c.Env == "real":
		return "/trading/info/portfolio"
	case pnl:
		return "/trading/info/demo/pnl"
	default:
		return "/trading/info/demo/portfolio"
	}
}

// FetchPortfolio returns the clientPortfolio object and the raw body.
func (c *Client) FetchPortfolio(ctx context.Context, pnl bool) (map[string]any, []byte, error) {
	v, raw, err := c.Get(ctx, c.PortfolioPath(pnl))
	if err != nil {
		return nil, raw, err
	}
	top, _ := v.(map[string]any)
	cp := Obj(top, "clientPortfolio")
	if cp == nil {
		cp = top
	}
	if cp == nil {
		cp = map[string]any{}
	}
	return cp, raw, nil
}

// Outcome of a trade request.
type Outcome string

const (
	// OutcomePlaced: the API answered 2xx with a parseable body.
	OutcomePlaced Outcome = "placed"
	// OutcomeFound: the response was ambiguous but the portfolio shows the trade.
	OutcomeFound Outcome = "found"
	// OutcomeNotFound: ambiguous response and the portfolio does not show the trade.
	OutcomeNotFound Outcome = "not-found"
	// OutcomeUnknown: ambiguous and the portfolio could not settle it.
	OutcomeUnknown Outcome = "unknown"
)

// Matcher inspects a portfolio and says whether the trade landed.
type Matcher func(cp map[string]any) (found bool, detail string)

// TradeResult is what PlaceTrade reports.
type TradeResult struct {
	Outcome Outcome
	Resp    *Response
	ID      string // order id when the response carried one
	Detail  string
	Cause   string
}

// PlaceTrade sends a trade request once. It never resends: after a timeout,
// reset or unparseable answer it waits, reads the PnL endpoint and uses match
// (when given) to decide whether the trade landed.
func (c *Client) PlaceTrade(ctx context.Context, r Request, match Matcher) (TradeResult, error) {
	r.Kind = KindTrade
	resp, err := c.Do(ctx, r)
	if err != nil {
		if amb, ok := err.(*AmbiguousError); ok {
			return c.reconcile(ctx, amb.Cause, match), nil
		}
		return TradeResult{}, err
	}
	v, derr := Decode(resp.Body)
	if derr != nil && len(strings.TrimSpace(string(resp.Body))) > 0 {
		return c.reconcile(ctx, "unparseable response", match), nil
	}
	res := TradeResult{Outcome: OutcomePlaced, Resp: resp}
	if v != nil {
		res.ID = DeepStr(v, "orderId", "orderID", "positionId", "positionID")
	}
	return res, nil
}

func (c *Client) reconcile(ctx context.Context, cause string, match Matcher) TradeResult {
	res := TradeResult{Outcome: OutcomeUnknown, Cause: cause}
	if match == nil {
		res.Detail = "no way to check automatically"
		return res
	}
	c.Sleep(c.ReconcileWait)
	cp, _, err := c.FetchPortfolio(ctx, true)
	if err != nil {
		res.Detail = "portfolio read failed: " + err.Error()
		return res
	}
	if ok, detail := match(cp); ok {
		res.Outcome, res.Detail = OutcomeFound, detail
	} else {
		res.Outcome, res.Detail = OutcomeNotFound, detail
	}
	return res
}

// Items returns positions, ordersForOpen, orders, ordersForClose of a clientPortfolio.
func Positions(cp map[string]any) []map[string]any { return Arr(cp, "positions") }

// OpenOrders returns ordersForOpen.
func OpenOrders(cp map[string]any) []map[string]any { return Arr(cp, "ordersForOpen") }

// LimitOrders returns the pending limit (market-if-touched) orders.
func LimitOrders(cp map[string]any) []map[string]any { return Arr(cp, "orders") }

// CloseOrders returns ordersForClose.
func CloseOrders(cp map[string]any) []map[string]any { return Arr(cp, "ordersForClose") }

// PortfolioIDs returns a set of "p:<id>" / "o:<id>" keys for every position and order.
func PortfolioIDs(cp map[string]any) map[string]bool {
	set := map[string]bool{}
	for _, p := range Positions(cp) {
		set["p:"+Str(p, "positionId", "positionID")] = true
	}
	for _, list := range [][]map[string]any{OpenOrders(cp), LimitOrders(cp), CloseOrders(cp)} {
		for _, o := range list {
			set["o:"+Str(o, "orderId", "orderID")] = true
		}
	}
	return set
}

// OpenMatcher finds a newly opened position or pending order for the instrument
// and side. baseline (may be nil) holds ids that existed before the request.
func OpenMatcher(instrumentID int64, isBuy bool, baseline map[string]bool, since string) Matcher {
	return func(cp map[string]any) (bool, string) {
		check := func(list []map[string]any, prefix, idKey, idKey2 string) (bool, string) {
			for _, o := range list {
				iid, _ := Int(o, "instrumentId", "instrumentID")
				buy, _ := Bool(o, "isBuy")
				if iid != instrumentID || buy != isBuy {
					continue
				}
				id := Str(o, idKey, idKey2)
				if baseline != nil {
					if baseline[prefix+id] {
						continue
					}
				} else if since != "" && Str(o, "openDateTime") < since {
					continue
				}
				if prefix == "p:" {
					return true, "position " + id
				}
				return true, "order " + id
			}
			return false, ""
		}
		if ok, d := check(Positions(cp), "p:", "positionId", "positionID"); ok {
			return true, d
		}
		for _, l := range [][]map[string]any{OpenOrders(cp), LimitOrders(cp)} {
			if ok, d := check(l, "o:", "orderId", "orderID"); ok {
				return true, d
			}
		}
		return false, "no new matching position or order in the portfolio"
	}
}

// CloseMatcher checks a full or partial close of positionID. baselineUnits is
// the unit count before the request (used for partial closes, <=0 for full).
func CloseMatcher(positionID string, partial bool, baselineUnits float64) Matcher {
	return func(cp map[string]any) (bool, string) {
		for _, p := range Positions(cp) {
			if Str(p, "positionId", "positionID") != positionID {
				continue
			}
			if partial {
				if u, ok := Num(p, "units"); ok && u < baselineUnits {
					return true, "position units reduced"
				}
			}
			for _, o := range CloseOrders(cp) {
				if Str(o, "positionId", "positionID") == positionID {
					return true, "close order pending"
				}
			}
			return false, "position " + positionID + " is still open"
		}
		return true, "position " + positionID + " is no longer open"
	}
}

// CancelMatcher checks an order is gone from every order list.
func CancelMatcher(orderID string) Matcher {
	return func(cp map[string]any) (bool, string) {
		for _, list := range [][]map[string]any{OpenOrders(cp), LimitOrders(cp), CloseOrders(cp)} {
			for _, o := range list {
				if Str(o, "orderId", "orderID") == orderID {
					return false, "order " + orderID + " is still pending"
				}
			}
		}
		return true, "order " + orderID + " is no longer pending"
	}
}

// Classification of a key pair.
type Classification struct {
	Env    string // demo | real | unknown
	Detail string
}

// Classify tells whether the key is real or demo by calling GET /trading/info/real/pnl:
// 200 is a real key, 403 InsufficientPermissions is a demo key. A 401 is returned as an error.
func (c *Client) Classify(ctx context.Context) (Classification, error) {
	cc := *c
	cc.NoGuard = true
	_, err := cc.Do(ctx, Request{Method: http.MethodGet, Path: "/trading/info/real/pnl"})
	if err == nil {
		return Classification{Env: "real"}, nil
	}
	if ae, ok := AsAPIError(err); ok {
		if ae.IsInsufficientPermissions() {
			return Classification{Env: "demo"}, nil
		}
		if ae.Status == 403 {
			return Classification{Env: "unknown", Detail: ae.Error()}, nil
		}
	}
	return Classification{}, err
}

// Permission tries to detect read/write from GET /api/v1/me scopes. It returns
// "read", "write" or "unknown"; it never places a trade.
func (c *Client) Permission(ctx context.Context) string {
	cc := *c
	cc.NoGuard = true
	v, _, err := cc.Get(ctx, "/me")
	if err != nil {
		return "unknown"
	}
	var scopes []string
	var walk func(any)
	walk = func(n any) {
		switch t := n.(type) {
		case string:
			if strings.Contains(t, ":read") || strings.Contains(t, ":write") {
				scopes = append(scopes, t)
			}
		case []any:
			for _, e := range t {
				walk(e)
			}
		case map[string]any:
			for _, e := range t {
				walk(e)
			}
		}
	}
	walk(v)
	if len(scopes) == 0 {
		return "unknown"
	}
	for _, s := range scopes {
		if strings.Contains(s, ":write") {
			return "write"
		}
	}
	return "read"
}
