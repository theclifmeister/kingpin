package protocol

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"sort"

	"github.com/theclifmeister/kingpin/internal/engine"
)

// Client is a front end's end of the protocol: a call by method name
// with positional params, its result decoded into result (nil to drop
// it), and every `event` notification received so far, as sent.
type Client interface {
	Call(method string, params []any, result any) error
	Events() []json.RawMessage
}

// RPCClient speaks the protocol over a writer and a reader: a kingpind
// process's stdin and stdout. A call writes its request and reads lines
// until its response, keeping the notifications that come first.
type RPCClient struct {
	w      io.Writer
	r      *bufio.Reader
	next   int
	events []json.RawMessage
	view   json.RawMessage
}

// NewRPCClient is a client writing requests to w and reading r.
func NewRPCClient(w io.Writer, r io.Reader) *RPCClient {
	return &RPCClient{w: w, r: bufio.NewReaderSize(r, 1<<20)}
}

// Events is every event notification received, as sent.
func (c *RPCClient) Events() []json.RawMessage { return c.events }

// LastView is the last view notification received, as sent.
func (c *RPCClient) LastView() json.RawMessage { return c.view }

// Call sends one request and waits for its response.
func (c *RPCClient) Call(method string, params []any, result any) error {
	c.next++
	if params == nil {
		params = []any{}
	}
	ps, err := json.Marshal(params)
	if err != nil {
		return err
	}
	id := fmt.Appendf(nil, "%d", c.next)
	req, err := json.Marshal(Request{JSONRPC: "2.0", ID: id, Method: method, Params: ps})
	if err != nil {
		return err
	}
	if _, err := c.w.Write(append(req, '\n')); err != nil {
		return err
	}
	for {
		line, err := c.r.ReadBytes('\n')
		if err != nil {
			return fmt.Errorf("%s: the server went away: %w", method, err)
		}
		var msg struct {
			Method string          `json:"method"`
			Params json.RawMessage `json:"params"`
			ID     json.RawMessage `json:"id"`
			Result json.RawMessage `json:"result"`
			Error  *Error          `json:"error"`
		}
		if err := json.Unmarshal(line, &msg); err != nil {
			return fmt.Errorf("%s: a line that is not JSON: %w", method, err)
		}
		switch msg.Method {
		case "event":
			c.events = append(c.events, msg.Params)
			continue
		case "view":
			c.view = msg.Params
			continue
		}
		if string(msg.ID) != string(id) {
			return fmt.Errorf("%s: answered %s, asked %s", method, msg.ID, id)
		}
		if msg.Error != nil {
			return msg.Error
		}
		if result != nil && len(msg.Result) > 0 {
			if raw, ok := result.(*json.RawMessage); ok {
				*raw = append((*raw)[:0], msg.Result...)
				return nil
			}
			return json.Unmarshal(msg.Result, result)
		}
		return nil
	}
}

// Play is the reference client's game (#300): it starts a run on the
// seed and plays it the way a greedy dealer does, through the protocol
// alone, until the run ends or its day reaches days (the view's day, not
// the loop's turns: a card answered is no day, #474). Each morning it reads the
// view, answers a card with its first choice, buys what the street
// connect where it stands sells with a share of the dirty cash, puts
// everything it holds on the street at the aggressive dial, and ends
// the day. A move the game refuses is part of play; any other error
// ends it. It returns the last view.
func Play(c Client, seed uint64, days int) (engine.View, error) {
	var v engine.View
	err := c.Call("new_run", []any{seed, "", false}, &v)
	if err != nil {
		return v, err
	}
	for {
		if v, err = view(c); err != nil {
			return v, err
		}
		if v.Over != nil || v.Day >= days {
			return v, nil
		}
		if v.Card != nil {
			if err := act(c, "choose", 0); err != nil {
				return v, err
			}
			continue
		}
		city := v.You.City
		for _, k := range v.Connects {
			if k.City != city || k.Wholesale || !k.Open {
				continue
			}
			products := make([]string, 0, len(k.Prices))
			for pid := range k.Prices {
				products = append(products, pid)
			}
			sort.Strings(products)
			for _, pid := range products {
				price := k.Prices[pid]
				if price <= 0 || len(products) == 0 {
					continue
				}
				qty := int(float64(v.You.DirtyCash) * 0.6 / float64(len(products)) / price)
				if qty > 0 {
					if err := act(c, "buy", k.ID, pid, qty, false); err != nil {
						return v, err
					}
				}
			}
		}
		if v, err = view(c); err != nil {
			return v, err
		}
		held := v.You.Stock[city]
		products := make([]string, 0, len(held))
		for pid := range held {
			products = append(products, pid)
		}
		sort.Strings(products)
		for _, pid := range products {
			if n := held[pid]; n > 0 {
				if err := act(c, "place_sell", city, pid, n, "aggressive"); err != nil {
					return v, err
				}
			}
		}
		if err := c.Call("end_day", nil, nil); err != nil {
			return v, err
		}
	}
}

// view is this morning's view, decoded into a fresh value: decoded over
// the last one, a field the new one leaves out (a card answered, omitted
// as empty) would keep the old value.
func view(c Client) (engine.View, error) {
	var v engine.View
	err := c.Call("view", nil, &v)
	return v, err
}

// act is a command whose refusal is part of play.
func act(c Client, method string, params ...any) error {
	if err := c.Call(method, params, nil); err != nil && !Refused(err) {
		return err
	}
	return nil
}
