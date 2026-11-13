package cli

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/NunoMotaRicardo/etoro-cli/internal/auth"
	"github.com/NunoMotaRicardo/etoro-cli/internal/etoro"
)

func side(o map[string]any) string {
	if b, ok := etoro.Bool(o, "isBuy"); ok {
		if b {
			return "buy"
		}
		return "sell"
	}
	return "-"
}

func positionPnL(p map[string]any) any {
	if u := etoro.Obj(p, "unrealizedPnL"); u != nil {
		if v, ok := etoro.Key(u, "pnL", "pnl"); ok {
			return v
		}
	}
	if v, ok := etoro.Key(p, "pnL"); ok {
		return v
	}
	return nil
}

func (a *App) portfolioDoc(cmd *cobra.Command, cl *etoro.Client, env string, withPnl bool) (*Doc, error) {
	cp, raw, err := cl.FetchPortfolio(cmd.Context(), withPnl)
	if err != nil {
		return nil, a.apiErr(err)
	}
	pos, oo, lo, co := etoro.Positions(cp), etoro.OpenOrders(cp), etoro.LimitOrders(cp), etoro.CloseOrders(cp)
	var idl []int64
	for _, l := range [][]map[string]any{pos, oo, lo, co} {
		for _, o := range l {
			id, _ := etoro.Int(o, "instrumentId", "instrumentID")
			idl = append(idl, id)
		}
	}
	names := cl.Names(cmd.Context(), a.symbols(), idl)
	sym := func(o map[string]any) any {
		id, _ := etoro.Int(o, "instrumentId", "instrumentID")
		if s := names[id]; s != "" {
			return s
		}
		return id
	}
	d := &Doc{Raw: [][]byte{raw}}
	d.KV = append(d.KV, KV{"environment", env})
	if v, ok := etoro.Key(cp, "credit"); ok {
		d.KV = append(d.KV, KV{"credit", v})
	}
	if v, ok := etoro.Key(cp, "unrealizedPnL"); ok && isScalar(v) {
		d.KV = append(d.KV, KV{"unrealizedPnL", v})
	}
	d.KV = append(d.KV, KV{"mirrors", len(etoro.Arr(cp, "mirrors"))})
	pt := Table{Name: "positions", Cols: []string{"positionId", "symbol", "side", "units", "amount", "leverage", "openRate", "stopLoss", "takeProfit"}}
	if withPnl {
		pt.Cols = append(pt.Cols, "pnl")
	}
	for _, p := range pos {
		row := []any{field(p, "positionId", "positionID"), sym(p), side(p), field(p, "units"), field(p, "amount"), field(p, "leverage"),
			field(p, "openRate"), field(p, "stopLossRate"), field(p, "takeProfitRate")}
		if withPnl {
			row = append(row, positionPnL(p))
		}
		pt.Rows = append(pt.Rows, row)
	}
	ot := Table{Name: "orders", Cols: []string{"orderId", "kind", "symbol", "side", "amount", "units", "rate", "leverage", "positionId"}}
	add := func(kind string, l []map[string]any) {
		for _, o := range l {
			ot.Rows = append(ot.Rows, []any{field(o, "orderId", "orderID"), kind, sym(o), side(o), field(o, "amount"),
				field(o, "amountInUnits", "units", "unitsToDeduct"), field(o, "rate"), field(o, "leverage"), field(o, "positionId", "positionID")})
		}
	}
	add("open", oo)
	add("limit", lo)
	add("close", co)
	d.Tables = []Table{pt, ot}
	return d, nil
}

func (a *App) cmdPortfolio(pnl bool) *cobra.Command {
	use, short := "portfolio", "Compact positions and pending orders (demo by default)"
	if pnl {
		use, short = "pnl", "Positions with live PnL and total unrealized PnL"
	}
	return &cobra.Command{
		Use: use, Short: short, Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			cl, env, err := a.client()
			if err != nil {
				return err
			}
			d, err := a.portfolioDoc(cmd, cl, env, pnl)
			if err != nil {
				return err
			}
			return a.emit(d)
		},
	}
}

func (a *App) cmdHistory() *cobra.Command {
	var since string
	var page, size int
	c := &cobra.Command{
		Use:   "history --since YYYY-MM-DD",
		Short: "Closed trade history (the real endpoint needs a real key)",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if _, err := time.Parse("2006-01-02", since); err != nil {
				return fail(ExitUsage, "--since must be YYYY-MM-DD")
			}
			cl, env, err := a.client()
			if err != nil {
				return err
			}
			path := "/trading/info/trade/history"
			if env == auth.Demo {
				path = "/trading/info/trade/demo/history"
			}
			q := []etoro.KV{{K: "minDate", V: since}, {K: "page", V: strconv.Itoa(page)}, {K: "pageSize", V: strconv.Itoa(size)}}
			v, raw, err := cl.Get(cmd.Context(), path, q...)
			if err != nil {
				if ae, ok := etoro.AsAPIError(err); ok && ae.IsInsufficientPermissions() {
					return fail(ExitError, "%s -- trade history needs a key for this environment: the real history endpoint rejects demo keys (InsufficientPermissions). Use `etoro-cli --real history` with a real key", ae.Error())
				}
				return a.apiErr(err)
			}
			return a.emit(autoDocWith(v, raw))
		},
	}
	c.Flags().StringVar(&since, "since", "", "earliest date, YYYY-MM-DD (required)")
	c.Flags().IntVar(&page, "page", 1, "page number")
	c.Flags().IntVar(&size, "page-size", 50, "rows per page")
	_ = c.MarkFlagRequired("since")
	return c
}

func autoDocWith(v any, raw []byte) *Doc { return autoDoc(v, raw) }

// ---- trading ----

func (a *App) execPath(env, tail string) string {
	if env == auth.Demo {
		return "/trading/execution/demo/" + tail
	}
	return "/trading/execution/" + tail
}

func fmtNum(f float64) string { return strconv.FormatFloat(f, 'f', -1, 64) }

type orderFlags struct {
	buy, sell     bool
	amount, units float64
	leverage      int
	sl, tp, rate  float64
	tsl           bool
}

func (o *orderFlags) bind(c *cobra.Command, withRate bool) {
	f := c.Flags()
	f.BoolVar(&o.buy, "buy", false, "open a long position")
	f.BoolVar(&o.sell, "sell", false, "open a short position")
	f.Float64Var(&o.amount, "amount", 0, "cash amount (USD) to invest")
	f.Float64Var(&o.units, "units", 0, "number of units")
	f.IntVar(&o.leverage, "leverage", 1, "leverage (default 1; never inferred)")
	f.Float64Var(&o.sl, "sl", 0, "stop-loss rate (omitted unless given)")
	f.Float64Var(&o.tp, "tp", 0, "take-profit rate (omitted unless given)")
	f.BoolVar(&o.tsl, "tsl", false, "enable trailing stop-loss (with --sl)")
	if withRate {
		f.Float64Var(&o.rate, "rate", 0, "trigger price (better than market: lower for buy, higher for sell)")
	}
}

func (o *orderFlags) validate(cmd *cobra.Command, withRate bool) error {
	if o.buy == o.sell {
		return fail(ExitUsage, "give exactly one of --buy or --sell")
	}
	if (cmd.Flags().Changed("amount")) == (cmd.Flags().Changed("units")) {
		return fail(ExitUsage, "give exactly one of --amount or --units")
	}
	if cmd.Flags().Changed("amount") && o.amount <= 0 || cmd.Flags().Changed("units") && o.units <= 0 {
		return fail(ExitUsage, "--amount/--units must be positive")
	}
	if o.leverage < 1 {
		return fail(ExitUsage, "--leverage must be >= 1")
	}
	if withRate && o.rate <= 0 {
		return fail(ExitUsage, "--rate is required for limit orders")
	}
	if o.tsl && !cmd.Flags().Changed("sl") {
		return fail(ExitUsage, "--tsl needs --sl")
	}
	return nil
}

func (o *orderFlags) body(cmd *cobra.Command, in etoro.Instrument, withRate bool) (map[string]any, string) {
	b := map[string]any{"InstrumentID": in.ID, "IsBuy": o.buy, "Leverage": o.leverage}
	size := ""
	if cmd.Flags().Changed("amount") {
		b["Amount"] = o.amount
		size = "amount $" + fmtNum(o.amount)
	} else {
		b["AmountInUnits"] = o.units
		size = "units " + fmtNum(o.units)
	}
	if withRate {
		b["Rate"] = o.rate
		size += " at rate " + fmtNum(o.rate)
	}
	if cmd.Flags().Changed("sl") {
		b["StopLossRate"] = o.sl
		size += " SL " + fmtNum(o.sl)
		if o.tsl {
			b["IsTslEnabled"] = true
			size += " (trailing)"
		}
	}
	if cmd.Flags().Changed("tp") {
		b["TakeProfitRate"] = o.tp
		size += " TP " + fmtNum(o.tp)
	}
	return b, size
}

func label(in etoro.Instrument, ref string) string {
	if in.Symbol != "" {
		return fmt.Sprintf("%s (id %d)", in.Symbol, in.ID)
	}
	return "id " + ref
}

func sideWord(buy bool) string {
	if buy {
		return "BUY"
	}
	return "SELL"
}

func (a *App) tradeReport(res etoro.TradeResult, verb string) error {
	switch res.Outcome {
	case etoro.OutcomePlaced:
		id := res.ID
		if id == "" {
			id = "(no id in response)"
		}
		fmt.Fprintf(a.Out, "ok: %s accepted, id %s. Verify with `etoro-cli portfolio`.\n", verb, id)
		if a.fRaw && res.Resp != nil {
			a.Out.Write(res.Resp.Body)
			fmt.Fprintln(a.Out)
		}
		return nil
	case etoro.OutcomeFound:
		fmt.Fprintf(a.Out, "ok: response was unclear (%s) but the portfolio shows the %s landed: %s. Nothing was resent.\n", res.Cause, verb, res.Detail)
		return nil
	case etoro.OutcomeNotFound:
		return fail(ExitError, "%s did not land: response was unclear (%s) and the portfolio shows no trace (%s). It was NOT resent; check `etoro-cli portfolio`, and only then ask whether to send it again as a new trade", verb, res.Cause, res.Detail)
	default:
		return fail(ExitAmbiguous, "%s outcome unknown (%s; %s). Do NOT resend. Run `etoro-cli portfolio` and look for it before doing anything else", verb, res.Cause, res.Detail)
	}
}

func (a *App) cmdTrade() *cobra.Command {
	t := &cobra.Command{Use: "trade", Short: "Open or close positions (needs --yes)"}
	of := &orderFlags{}
	open := &cobra.Command{
		Use:   "open <SYMBOL|id> --buy|--sell --amount N|--units N",
		Short: "Open a market position (leverage defaults to 1; SL/TP only if given)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := of.validate(cmd, false); err != nil {
				return err
			}
			cl, env, err := a.client()
			if err != nil {
				return err
			}
			in, err := cl.Resolve(cmd.Context(), a.symbols(), args[0])
			if err != nil {
				return a.apiErr(err)
			}
			body, size := of.body(cmd, in, false)
			tail := "market-open-orders/by-amount"
			if cmd.Flags().Changed("units") {
				tail = "market-open-orders/by-units"
			}
			if err := a.gate(env, fmt.Sprintf("%s %s, %s, leverage x%d", sideWord(of.buy), label(in, args[0]), size, of.leverage)); err != nil {
				return err
			}
			var baseline map[string]bool
			if cp, _, err := cl.FetchPortfolio(cmd.Context(), true); err == nil {
				baseline = etoro.PortfolioIDs(cp)
			}
			since := time.Now().UTC().Add(-30 * time.Second).Format("2006-01-02T15:04:05")
			b, _ := jsonMarshal(body)
			res, err := cl.PlaceTrade(cmd.Context(), etoro.Request{Method: "POST", Path: a.execPath(env, tail), Body: b}, etoro.OpenMatcher(in.ID, of.buy, baseline, since))
			if err != nil {
				return a.apiErr(err)
			}
			return a.tradeReport(res, "open order")
		},
	}
	of.bind(open, false)

	var units float64
	closeC := &cobra.Command{
		Use:   "close <positionId> [--units N]",
		Short: "Close a position by id (full, or partially with --units)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			partial := cmd.Flags().Changed("units")
			if partial && units <= 0 {
				return fail(ExitUsage, "--units must be positive")
			}
			pid := args[0]
			if _, err := strconv.ParseInt(pid, 10, 64); err != nil {
				return fail(ExitUsage, "close takes a numeric positionId (see `etoro-cli portfolio`), not a symbol")
			}
			cl, env, err := a.client()
			if err != nil {
				return err
			}
			cp, _, err := cl.FetchPortfolio(cmd.Context(), true)
			if err != nil {
				return a.apiErr(err)
			}
			var pos map[string]any
			for _, p := range etoro.Positions(cp) {
				if etoro.Str(p, "positionId", "positionID") == pid {
					pos = p
				}
			}
			if pos == nil {
				return fail(ExitError, "position %s not found in the %s portfolio", pid, env)
			}
			iid, _ := etoro.Int(pos, "instrumentId", "instrumentID")
			have, _ := etoro.Num(pos, "units")
			names := cl.Names(cmd.Context(), a.symbols(), []int64{iid})
			in := etoro.Instrument{ID: iid, Symbol: names[iid]}
			what := "close ALL of position " + pid
			body := map[string]any{"InstrumentId": iid, "UnitsToDeduct": nil}
			if partial {
				what = fmt.Sprintf("close %s units of position %s", fmtNum(units), pid)
				body["UnitsToDeduct"] = units
			}
			if err := a.gate(env, fmt.Sprintf("%s (%s, %s, holding %s units)", what, label(in, strconv.FormatInt(iid, 10)), side(pos), fmtNum(have))); err != nil {
				return err
			}
			b, _ := jsonMarshal(body)
			res, err := cl.PlaceTrade(cmd.Context(), etoro.Request{Method: "POST", Path: a.execPath(env, "market-close-orders/positions/"+pid), Body: b},
				etoro.CloseMatcher(pid, partial, have))
			if err != nil {
				return a.apiErr(err)
			}
			return a.tradeReport(res, "close order")
		},
	}
	closeC.Flags().Float64Var(&units, "units", 0, "units to close (default: the whole position)")
	t.AddCommand(open, closeC)
	return t
}

func (a *App) cmdOrder() *cobra.Command {
	o := &cobra.Command{Use: "order", Short: "Limit (market-if-touched) orders and cancellations (needs --yes)"}
	of := &orderFlags{}
	limit := &cobra.Command{
		Use:   "limit <SYMBOL|id> --buy|--sell --rate R --amount N|--units N",
		Short: "Place a limit order that triggers at --rate",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := of.validate(cmd, true); err != nil {
				return err
			}
			cl, env, err := a.client()
			if err != nil {
				return err
			}
			in, err := cl.Resolve(cmd.Context(), a.symbols(), args[0])
			if err != nil {
				return a.apiErr(err)
			}
			body, size := of.body(cmd, in, true)
			if err := a.gate(env, fmt.Sprintf("LIMIT %s %s, %s, leverage x%d", sideWord(of.buy), label(in, args[0]), size, of.leverage)); err != nil {
				return err
			}
			var baseline map[string]bool
			if cp, _, err := cl.FetchPortfolio(cmd.Context(), true); err == nil {
				baseline = etoro.PortfolioIDs(cp)
			}
			b, _ := jsonMarshal(body)
			res, err := cl.PlaceTrade(cmd.Context(), etoro.Request{Method: "POST", Path: a.execPath(env, "limit-orders"), Body: b},
				etoro.OpenMatcher(in.ID, of.buy, baseline, ""))
			if err != nil {
				return a.apiErr(err)
			}
			return a.tradeReport(res, "limit order")
		},
	}
	of.bind(limit, true)

	var kind string
	cancel := &cobra.Command{
		Use:   "cancel <orderId> --kind open|close|limit",
		Short: "Cancel a pending order",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			tail := map[string]string{"open": "market-open-orders/", "close": "market-close-orders/", "limit": "limit-orders/"}[kind]
			if tail == "" {
				return fail(ExitUsage, "--kind must be open, close or limit")
			}
			if _, err := strconv.ParseInt(args[0], 10, 64); err != nil {
				return fail(ExitUsage, "orderId must be numeric")
			}
			cl, env, err := a.client()
			if err != nil {
				return err
			}
			if err := a.gate(env, fmt.Sprintf("cancel %s order %s", kind, args[0])); err != nil {
				return err
			}
			res, err := cl.PlaceTrade(cmd.Context(), etoro.Request{Method: "DELETE", Path: a.execPath(env, tail+args[0])}, etoro.CancelMatcher(args[0]))
			if err != nil {
				return a.apiErr(err)
			}
			return a.tradeReport(res, "cancellation")
		},
	}
	cancel.Flags().StringVar(&kind, "kind", "", "open, close or limit (required)")
	_ = cancel.MarkFlagRequired("kind")
	o.AddCommand(limit, cancel)
	return o
}

func jsonMarshal(v any) ([]byte, error) {
	var sb strings.Builder
	enc := newEnc(&sb)
	if err := enc.Encode(v); err != nil {
		return nil, err
	}
	return []byte(strings.TrimSpace(sb.String())), nil
}
