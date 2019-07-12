package main

import (
	"database/sql"
	"database/sql/driver"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/Krognol/tbapi"
	"github.com/Krognol/thronebot/internal"
	"github.com/Krognol/thronebot/internal/router"
	"github.com/bwmarrin/discordgo"
	_ "github.com/mattn/go-sqlite3"
)

type config struct {
	ArchiveRepo      string `json:"archive_repo"`
	DatabasePath     string `json:"database_path"`
	LogPath          string `json:"log_path"`
	WeeklySuggestion string `json:"weekly_suggestion"`
	WeeklyVoting     string `json:"weekly_voting"`
	Staff            string `json:"staff"`
}

var (
	tbAPIKey      = flag.String("tb", "", "Thronebot API key")
	discordBotKey = flag.String("t", "", "Discord bot key")
	githubAPIKey  = flag.String("git", "", "Github API key. For archiving of pins")
	configPath    = flag.String("cfg", "config.json", "Path to config file.")
)

func main() {
	cfg := new(config)
	func() {
		f, err := os.Open(*configPath)
		if err != nil {
			panic(err)
		}
		defer f.Close()

		err = json.NewDecoder(f).Decode(cfg)
		if err != nil {
			panic(err)
		}
	}()

	if cfg.LogPath == "" {
		log.SetOutput(os.Stdout)
	} else {
		f, err := os.Open(cfg.LogPath)
		if err != nil {
			// Can't open/create file, something's wrong
			panic(err)
		}
		log.SetOutput(f)
		defer f.Close()
	}

	if *discordBotKey == "" {
		log.Fatal("Discord bot key can't be nil:", flag.ErrHelp)
	}

	if *tbAPIKey == "" {
		log.Fatal("Missing Thronebutt API key.")
	}

	db, err := sql.Open("sqlite3", cfg.DatabasePath)
	if err != nil {
		log.Fatal(err)
	}

	defer db.Close()

	ses, err := discordgo.New(*discordBotKey)
	if err != nil {
		log.Fatal(err)
	}

	defer ses.Close()

	bot := internal.NewBot(ses, db, router.NewRoute())

	tbClient := tbapi.New(*tbAPIKey)
	// Commands
	bot.Route.On("config", func(ctx *router.Context) {
		ctx.Reply(
			"Current config settings\n  Staff:", cfg.Staff,
			"\n  Weekly voting:", cfg.WeeklyVoting,
			"\n  Weekly suggestion:", cfg.WeeklySuggestion,
		)
	}).On("set", cfgSetHandler(cfg))

	bot.Route.On("pingdb", pingdbHandler(bot.DB)).Use(internal.ElevatedUser).Desc("Pings the database for a connection.")

	bot.Route.On("weekly", nil).
		On("suggest", weeklySuggestionHandler(bot.DB)).
		Desc("Suggest a weekly. Ex. `steroids/b/grenade launcher/crown of death`").
		On("banned", internal.GetBannedHandler(bot.DB)).Desc("Print banned selections.").
		// TODO
		On("ban", weeklyBanUnbanHandler(bot.DB)).Use(internal.ElevatedUser).
		On("enable", weeklyEnableDisableHandler(tbClient, true)).Use(internal.ElevatedUser).
		On("disable", weeklyEnableDisableHandler(tbClient, false)).Use(internal.ElevatedUser).
		On("set", nil).Use(internal.ElevatedUser)

	if *githubAPIKey != "" {
		// TODO register archiving routes
	}

	var botID = ses.State.User.ID

	bot.Ses.AddHandler(func(s *discordgo.Session, m *discordgo.MessageCreate) {
		bot.Route.FindAndExecute(s, "thronebot", botID, m.Message)
	})

	closer := make(chan os.Signal, 1)
	signal.Notify(closer, syscall.SIGINT, syscall.SIGTERM, os.Interrupt, os.Kill)

	fmt.Println("Bot is running. Ctrl+C to quit.")
	<-closer

	f, err := os.Open(*configPath)
	if err != nil {
		log.Println("failed to open config file:", err)
		os.Exit(1)
	}

	err = json.NewEncoder(f).Encode(cfg)
	if err != nil {
		log.Println("failed to save config to file:", err)
		os.Exit(1)
	}
}

func weeklyBanUnbanHandler(db *sql.DB) router.HandlerFunc {
	return func(ctx *router.Context) {
		if len(ctx.Args) < 3 {
			ctx.Reply(
				"Usage: `thronebot weekly ban [add|del] [crown|char|wep] (name)`\n",
				"To ban an item: ex: `thronebot weekly ban add crown crown of blood\n",
				"To unban an item: ex: `thronebot weekly ban del char steroids",
			)
			return
		}

		addel := ctx.Args.Get(0)
		kind := ctx.Args.Get(1)
		which := ctx.Args.After(2)
		var err error
		var val int

		switch kind {
		case "char":
			val = internal.Chars.NameToID(which)
		case "crown":
			val = internal.Crowns.NameToID(which)
		case "wep":
			val = internal.Weapons.NameToID(which)
		default:
			ctx.Reply("Invalid option: ", which)
			return
		}

		if val == -1 {
			ctx.Reply("Invalid selection: ", which)
			return
		}

		switch addel {
		case "add":
			err = internal.WeeklyBanAdd(db, kind, val)
		case "del":
			err = internal.WeeklyBanDel(db, kind, val)
		default:
			ctx.Reply("Invalid option: ", addel, "\n Expected `add` or `del`")
			return
		}

		if err != nil {
			ctx.Reply("Failed to ban item: ", err)
		}
	}
}

func cfgSetHandler(cfg *config) router.HandlerFunc {
	return func(ctx *router.Context) {
		prop, val := ctx.Args.Get(0), ctx.Args.Get(1)
		if prop == "" || val == "" {
			ctx.Reply("Missing property name or value")
			return
		}

		switch prop {
		case "weekly_suggestion":
			cfg.WeeklySuggestion = val
		case "weekly_voting":
			cfg.WeeklyVoting = val
		case "staff":
			cfg.Staff = val
		default:
			ctx.Reply("Invalid property name")
			return
		}
		log.Println("cfgSet: set property '", prop, "' to value '", val, "'")
		ctx.Reply("Set ", prop, " to, ", val)
	}
}

func weeklyEnableDisableHandler(tbc *tbapi.Client, enable bool) router.HandlerFunc {
	// enable true, disable false
	return func(ctx *router.Context) {
		var (
			res *http.Response
			err error
		)

		if enable {
			res, err = tbc.EnableWeekly()
		} else {
			res, err = tbc.DisableWeekly()
		}

		defer res.Body.Close()
		if err != nil {
			log.Println("weeklyEnableDisable: error enabling/disabling weekly:", err)
		}
		ctx.Reply("Got response:", res.Status)
	}
}

func pingdbHandler(db *sql.DB) router.HandlerFunc {
	return func(ctx *router.Context) {
		res := db.Ping()
		if res == driver.ErrBadConn {
			log.Println("bot: bad DB conn:", res)
		}
		ctx.Reply(res.Error())
	}
}

func weeklySuggestionHandler(db *sql.DB) router.HandlerFunc {
	return func(ctx *router.Context) {
		suggestCount := internal.GetUserSuggestionCount(db, ctx.Msg.Author.ID)
		if suggestCount >= 3 {
			ctx.Reply("You've already made 3 suggestions this week.")
			return
		}

		build := strings.Split(strings.ToLower(ctx.Args.Get(0)), "/")
		if len(build) < 4 {
			ctx.Reply(ctx.Msg.Author.Mention(), "Too few arguments")
			return
		}

		var (
			char  = build[0]
			skin  = build[1] == "b"
			weap  = build[2]
			crown = build[3]
		)

		if _, ok := internal.Chars[char]; !ok {
			ctx.Reply("Invalid character")
			return
		}

		if _, ok := internal.Weapons[weap]; !ok {
			ctx.Reply("Invalid weapon")
			return
		}

		if _, ok := internal.Crowns[crown]; !ok {
			ctx.Reply("Invalid crown")
			return
		}

		somethingBanned, err := internal.IsBanned(db, build[0], build[2], build[3])
		if err != nil {
			ctx.Reply("Error while checking for banned items.")
			return
		}

		if somethingBanned {
			ctx.Reply("One or more of your selections are currently banned. Remember to check the banned list for banned items every week.")
			return
		}

		err = internal.InsertSuggestion(db, ctx.Msg.Author.ID, char, weap, crown, skin)
		if err != nil {
			ctx.Reply("Failed to save suggestion.")
		} else {
			// TODO post to weekly voting channel
		}
	}
}
