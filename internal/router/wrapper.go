package router

import (
	"errors"
	"strings"

	"github.com/bwmarrin/discordgo"
	"github.com/necroforger/dgrouter"
)

var errRouteNotFound = errors.New("router: Could not find route")

type (
	// HandlerFunc ...
	HandlerFunc func(*Context)

	// MiddlewareFunc ...
	MiddlewareFunc func(HandlerFunc) HandlerFunc

	// Route router wrapper
	Route struct {
		*dgrouter.Route
	}
)

// NewRoute ...
func NewRoute() *Route {
	return &Route{dgrouter.New()}
}

// On matches a Route with a name
func (r *Route) On(name string, handler HandlerFunc) *Route {
	return &Route{r.Route.On(name, WrapHandler(handler))}
}

// Group groups multiple routes together
func (r *Route) Group(fn func(rt *Route)) *Route {
	return &Route{r.Route.Group(func(r *dgrouter.Route) {
		fn(&Route{r})
	})}
}

// Use specifes what MiddlewareFuncs a Route should have
func (r *Route) Use(mfn ...MiddlewareFunc) *Route {
	wrapped := make([]dgrouter.MiddlewareFunc, len(mfn))
	for i, fn := range mfn {
		wrapped[i] = WrapMiddleware(fn)
	}

	return &Route{
		r.Route.Use(wrapped...),
	}
}

// WrapMiddleware wraps a MiddlewareFunc into a dgrouter.MiddlewareFunc
func WrapMiddleware(mfn MiddlewareFunc) dgrouter.MiddlewareFunc {
	return func(next dgrouter.HandlerFunc) dgrouter.HandlerFunc {
		return func(i interface{}) {
			WrapHandler(mfn(UnwrapHandler(next)))(i)
		}
	}
}

// WrapHandler wraps a HandlerFunc into a dgrouter.HandlerFunc
func WrapHandler(fn HandlerFunc) dgrouter.HandlerFunc {
	if fn == nil {
		return nil
	}

	return func(i interface{}) {
		fn(i.(*Context))
	}
}

// UnwrapHandler unwraps a dgrouter.HandlerFunc into a HandlerFunc
func UnwrapHandler(fn dgrouter.HandlerFunc) HandlerFunc {
	return func(ctx *Context) {
		fn(ctx)
	}
}

func mention(s string) string {
	return "<@" + s + ">"
}

func nickMention(s string) string {
	return "<@!" + s + ">"
}

// FindAndExecute finds the closest command and executes the callback
func (r *Route) FindAndExecute(s *discordgo.Session, prefix string, botID string, m *discordgo.Message) error {
	var pf string

	if r.Default != nil && (m.Content == mention(botID) || m.Content == nickMention(botID)) {
		r.Default.Handler(NewContext(s, m, ParseArgs(m.Content), r.Default))
		return nil
	}

	bmention := mention(botID)
	nmention := nickMention(botID)

	p := func(t string) bool {
		return strings.HasPrefix(m.Content, t)
	}

	switch {
	case prefix != "" && p(prefix):
		pf = prefix
	case p(bmention):
		pf = bmention
	case p(nmention):
		pf = nmention
	default:
		return errRouteNotFound
	}

	command := strings.TrimPrefix(m.Content, pf)
	args := ParseArgs(command)

	if rt, depth := r.FindFull(args...); depth > 0 {
		args = append([]string{strings.Join(args[:depth], string(separator))}, args[depth:]...)
		rt.Handler(NewContext(s, m, args, rt))
	} else {
		return errRouteNotFound
	}

	return nil
}
