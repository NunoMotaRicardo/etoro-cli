package cli

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/spf13/cobra"

	"github.com/NunoMotaRicardo/etoro-cli/internal/etoro"
)

func (a *App) cmdSearch() *cobra.Command {
	var exact bool
	var page int
	c := &cobra.Command{
		Use:   "search <text>",
		Short: "Search instruments (--exact: exact symbol lookup)",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cl, _, err := a.client()
			if err != nil {
				return err
			}
			text := strings.Join(args, " ")
			q := []etoro.KV{{K: "fields", V: etoro.SearchFields}}
			if exact {
				q = append(q, etoro.KV{K: "internalSymbolFull", V: strings.ToUpper(text)})
			} else {
				q = append(q, etoro.KV{K: "searchText", V: text})
			}
			size := 10
			if a.fLimit > 0 {
				size = a.fLimit
			}
			q = append(q, etoro.KV{K: "pageSize", V: strconv.Itoa(size)}, etoro.KV{K: "pageNumber", V: strconv.Itoa(page)})
			v, raw, err := cl.Get(cmd.Context(), "/market-data/search", q...)
			if err != nil {
				return a.apiErr(err)
			}
			t := Table{Cols: []string{"id", "symbol", "name", "market"}}
			var ins []etoro.Instrument
			for _, o := range etoro.FindArray(v) {
				in := etoro.Instrument{}
				in.ID, _ = etoro.Int(o, "instrumentId", "instrumentID")
				in.Symbol = etoro.Str(o, "internalSymbolFull", "symbolFull")
				in.Name = etoro.Str(o, "displayname", "displayName")
				in.MarketID, _ = etoro.Int(o, "marketId", "marketID")
				if exact && !strings.EqualFold(in.Symbol, text) {
					continue
				}
				ins = append(ins, in)
				t.Rows = append(t.Rows, []any{in.ID, in.Symbol, in.Name, in.MarketID})
			}
			a.symbols().Put(ins...)
			return a.emit(&Doc{Tables: []Table{t}, Raw: [][]byte{raw}})
		},
	}
	c.Flags().BoolVar(&exact, "exact", false, "exact symbol match (internalSymbolFull)")
	c.Flags().IntVar(&page, "page", 1, "page number")
	return c
}

func (a *App) resolveAll(cmd *cobra.Command, cl *etoro.Client, refs []string) ([]etoro.Instrument, error) {
	var out []etoro.Instrument
	for _, r := range refs {
		in, err := cl.Resolve(cmd.Context(), a.symbols(), r)
		if err != nil {
			return nil, a.apiErr(err)
		}
		out = append(out, in)
	}
	return out, nil
}

func ids(ins []etoro.Instrument) []int64 {
	out := make([]int64, len(ins))
	for i, in := range ins {
		out[i] = in.ID
	}
	return out
}

func (a *App) cmdQuote() *cobra.Command {
	return &cobra.Command{
		Use:   "quote <SYMBOL|id>...",
		Short: "Live bid/ask rates (symbols are resolved and cached)",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cl, _, err := a.client()
			if err != nil {
				return err
			}
			ins, err := a.resolveAll(cmd, cl, args)
			if err != nil {
				return err
			}
			bodies, raws, err := cl.GetIDs(cmd.Context(), "/market-data/instruments/rates", "instrumentIds", ids(ins), nil, 100)
			if err != nil {
				return a.apiErr(err)
			}
			names := map[int64]string{}
			for _, in := range ins {
				names[in.ID] = in.Symbol
			}
			t := Table{Cols: []string{"id", "symbol", "bid", "ask", "last", "time"}}
			for _, b := range bodies {
				for _, o := range etoro.FindArray(b) {
					id, _ := etoro.Int(o, "instrumentId", "instrumentID")
					t.Rows = append(t.Rows, []any{id, names[id], field(o, "bid"), field(o, "ask"), field(o, "lastExecution"), field(o, "date")})
				}
			}
			return a.emit(&Doc{Tables: []Table{t}, Raw: raws})
		},
	}
}

func field(o map[string]any, k ...string) any {
	v, _ := etoro.Key(o, k...)
	return v
}

func (a *App) cmdInstrument() *cobra.Command {
	return &cobra.Command{
		Use:   "instrument <SYMBOL|id>",
		Short: "Display data for one instrument",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cl, _, err := a.client()
			if err != nil {
				return err
			}
			in, err := cl.Resolve(cmd.Context(), a.symbols(), args[0])
			if err != nil {
				return a.apiErr(err)
			}
			v, raw, err := cl.Get(cmd.Context(), "/market-data/instruments", etoro.KV{K: "instrumentIds", V: strconv.FormatInt(in.ID, 10)})
			if err != nil {
				return a.apiErr(err)
			}
			objs := etoro.FindArray(v)
			if len(objs) == 0 {
				return fail(ExitError, "instrument %d not found", in.ID)
			}
			d := &Doc{Raw: [][]byte{raw}}
			for _, kv := range FlatScalars(objs[0]) {
				d.KV = append(d.KV, kv)
			}
			return a.emit(d)
		},
	}
}

var intervals = map[string]string{
	"1m": "OneMinute", "5m": "FiveMinutes", "10m": "TenMinutes", "15m": "FifteenMinutes", "30m": "ThirtyMinutes",
	"1h": "OneHour", "4h": "FourHours", "1d": "OneDay", "1w": "OneWeek",
}

func (a *App) cmdCandles() *cobra.Command {
	var interval, dir string
	var count int
	c := &cobra.Command{
		Use:   "candles <SYMBOL|id>",
		Short: "OHLCV candles (--interval 1m|5m|10m|15m|30m|1h|4h|1d|1w or the API name)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if dir != "asc" && dir != "desc" {
				return fail(ExitUsage, "--dir must be asc or desc")
			}
			if count < 1 || count > 1000 {
				return fail(ExitUsage, "--count must be 1..1000")
			}
			if full, ok := intervals[strings.ToLower(interval)]; ok {
				interval = full
			}
			cl, _, err := a.client()
			if err != nil {
				return err
			}
			in, err := cl.Resolve(cmd.Context(), a.symbols(), args[0])
			if err != nil {
				return a.apiErr(err)
			}
			path := fmt.Sprintf("/market-data/instruments/%d/history/candles/%s/%s/%d", in.ID, dir, interval, count)
			v, raw, err := cl.Get(cmd.Context(), path)
			if err != nil {
				return a.apiErr(err)
			}
			t := Table{Cols: []string{"date", "open", "high", "low", "close", "volume"}}
			for _, o := range etoro.FindArray(v) {
				t.Rows = append(t.Rows, []any{field(o, "fromDate", "date", "time"), field(o, "open"), field(o, "high"), field(o, "low"), field(o, "close"), field(o, "volume")})
			}
			return a.emit(&Doc{Tables: []Table{t}, Raw: [][]byte{raw}})
		},
	}
	f := c.Flags()
	f.StringVar(&interval, "interval", "1d", "candle interval")
	f.IntVar(&count, "count", 30, "number of candles (max 1000)")
	f.StringVar(&dir, "dir", "desc", "asc or desc")
	return c
}

func (a *App) cmdClosingPrices() *cobra.Command {
	return &cobra.Command{
		Use:   "closing-prices <SYMBOL|id>...",
		Short: "Historical closing prices for instruments",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cl, _, err := a.client()
			if err != nil {
				return err
			}
			ins, err := a.resolveAll(cmd, cl, args)
			if err != nil {
				return err
			}
			bodies, raws, err := cl.GetIDs(cmd.Context(), "/market-data/instruments/history/closing-price", "instrumentIds", ids(ins), nil, 100)
			if err != nil {
				return a.apiErr(err)
			}
			d := &Doc{Raw: raws}
			for _, b := range bodies {
				sub := autoDoc(b, nil)
				d.KV = append(d.KV, sub.KV...)
				d.Tables = append(d.Tables, sub.Tables...)
			}
			return a.emit(d)
		},
	}
}
