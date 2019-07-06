package internal

import (
	"database/sql"

	"github.com/Krognol/thronebot/internal/router"
	"github.com/bwmarrin/discordgo"
)

// Bot ...
type Bot struct {
	Ses   *discordgo.Session
	DB    *sql.DB
	Route *router.Route
}

// NewBot returns a new Discord bot
func NewBot(ses *discordgo.Session, db *sql.DB, route *router.Route) *Bot {
	return &Bot{
		Ses:   ses,
		DB:    db,
		Route: route,
	}
}
