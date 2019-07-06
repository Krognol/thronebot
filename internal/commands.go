package internal

import (
	"log"

	"github.com/Krognol/thronebot/internal/router"
	"github.com/bwmarrin/discordgo"
)

// ElevatedUser checks for elevated admin permissions or bot owner ID
func ElevatedUser(fn router.HandlerFunc) router.HandlerFunc {
	return func(ctx *router.Context) {
		if ctx.Msg.Author.ID == "95957677376540672" {
			fn(ctx)
			return
		}

		perms, err := ctx.Ses.UserChannelPermissions(ctx.Msg.Author.ID, ctx.Msg.ChannelID)
		if err != nil {
			log.Print("commands: failed to retrieve channel permissions:", err)
			ctx.Reply("Could not retrieve channel permissions: ", err)
			return
		}

		if (perms&discordgo.PermissionAll != 0) || (perms&discordgo.PermissionAllChannel != 0) {
			fn(ctx)
			return
		}
	}
}
