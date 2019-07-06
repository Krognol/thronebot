package router

import (
	"fmt"
	"sync"

	"github.com/bwmarrin/discordgo"
	"github.com/necroforger/dgrouter"
)

// Context ...
type Context struct {
	Route *dgrouter.Route
	Msg   *discordgo.Message
	Ses   *discordgo.Session

	Args Args

	Vars *sync.Map
}

// Set stores a value in the Vars map
func (c *Context) Set(key string, v interface{}) { c.Vars.Store(key, v) }

// Get gets a value from the Vars map
func (c *Context) Get(key string) (interface{}, bool) { return c.Vars.Load(key) }

// Reply sends a normal message to the channel the parent message was sent in
func (c *Context) Reply(args ...interface{}) {
	c.Ses.ChannelMessageSend(c.Msg.ChannelID, fmt.Sprint(args...))
}

// ReplyEmbed same as Reply but sends and Embed
func (c *Context) ReplyEmbed(e *discordgo.MessageEmbed) {
	c.Ses.ChannelMessageSendEmbed(c.Msg.ChannelID, e)
}

// ReplyEmbedQuick same as ReplyEmbed but has only the Description property
func (c *Context) ReplyEmbedQuick(args ...interface{}) {
	c.Ses.ChannelMessageSendEmbed(
		c.Msg.ChannelID,
		&discordgo.MessageEmbed{
			Description: fmt.Sprint(args...),
		},
	)
}

// Guild retrieves a guild from the state or restapi
func (c *Context) Guild(guildID string) (*discordgo.Guild, error) {
	g, err := c.Ses.State.Guild(guildID)
	if err != nil {
		g, err = c.Ses.Guild(guildID)
	}
	return g, err
}

// Channel retrieves a channel from the state or restapi
func (c *Context) Channel(channelID string) (*discordgo.Channel, error) {
	ch, err := c.Ses.State.Channel(channelID)
	if err != nil {
		ch, err = c.Ses.Channel(channelID)
	}
	return ch, err
}

// Member retrieves a member from the state or restapi
func (c *Context) Member(guildID, userID string) (*discordgo.Member, error) {
	m, err := c.Ses.State.Member(guildID, userID)
	if err != nil {
		m, err = c.Ses.GuildMember(guildID, userID)
	}
	return m, err
}

// NewContext returns a new context from a message
func NewContext(s *discordgo.Session, m *discordgo.Message, args Args, route *dgrouter.Route) *Context {
	return &Context{
		Route: route,
		Msg:   m,
		Ses:   s,
		Args:  args,
		Vars:  &sync.Map{},
	}
}
