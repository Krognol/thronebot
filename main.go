package main

import (
	"database/sql"
	"database/sql/driver"
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

var (
	tbAPIKey          = flag.String("tb", "", "Thronebot API key")
	discordBotKey     = flag.String("t", "", "Discord bot key")
	githubAPIKey      = flag.String("git", "", "Github API key. For archiving of pins")
	githubArchiveRepo = flag.String("rep", "", "Repo name")
	dbName            = flag.String("db", "db.sqlite", "Path to SQLite DB file")
	logFile           = flag.String("log", "", "Path to file to print logging to")
)

func main() {
	if *logFile == "" {
		log.SetOutput(os.Stdout)
	} else {
		f, err := os.Open(*logFile)
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

	db, err := sql.Open("sqlite3", *dbName)
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
	bot.Route.On("pingdb", pingdbHandler(bot.DB)).Use(internal.ElevatedUser).Desc("Pings the database for a connection.")

	bot.Route.On("weekly", nil).
		On("suggest", weeklySuggestionHandler(bot.DB)).
		Desc("Suggest a weekly. Ex. `steroids/b/grenade launcher/crown of death`").
		// TODO print banned things
		On("banned", nil).Desc("Print banned selections.").
		// TODO
		On("ban", nil).Use(internal.ElevatedUser).
		On("enable", weeklyEnableDisableHandler(tbClient, true)).Use(internal.ElevatedUser).
		On("disable", weeklyEnableDisableHandler(tbClient, false)).Use(internal.ElevatedUser).
		On("set", nil).Use(internal.ElevatedUser)

	if *githubAPIKey != "" {
		// TODO register archiving routes
	}

	var BotID = ses.State.User.ID

	bot.Ses.AddHandler(func(s *discordgo.Session, m *discordgo.MessageCreate) {
		bot.Route.FindAndExecute(s, "thronebot", BotID, m.Message)
	})

	closer := make(chan os.Signal, 1)
	signal.Notify(closer, syscall.SIGINT, syscall.SIGTERM, os.Interrupt, os.Kill)

	fmt.Println("Bot is running. Ctrl+C to quit.")
	<-closer
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

func getBannedHandler(db *sql.DB) router.HandlerFunc {
	return func(ctx *router.Context) {
		rows, err := db.Query("SELECT * FROM weekly_banned;")
		if err != nil {
			log.Println("getBanned: failed to query db:", err)
			ctx.Reply("Failed to retrieve banned items.")
			return
		}

		defer rows.Close()

		var (
			char, crown, wep    string
			chars, crowns, weps []string
		)

		// TODO find more efficient solution
		for rows.Next() {
			err = rows.Scan(&char, &crown, &wep)
			if err != nil {
				log.Fatal("getBanned: failed to scan row:", err)
				ctx.Reply("Failed to retrieve banned items.")
				return
			}

			chars = append(chars, char)
			crowns = append(crowns, crown)
			weps = append(weps, wep)
		}

		ctx.Reply(
			"Currently banned items:\n\n**Characters:**\n  ", strings.Join(chars, ", "),
			"\n**Crowns:**\n  ", strings.Join(crowns, ", "),
			"**Weapons:**\n  ", strings.Join(weps, ", "),
		)
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
		suggestCount := getUserSuggestionCount(db, ctx.Msg.Author.ID)
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

		somethingBanned, err := isBanned(db, build[0], build[2], build[3])
		if err != nil {
			ctx.Reply("Error while checking for banned items.")
			return
		}

		if somethingBanned {
			ctx.Reply("One or more of your selections are currently banned. Remember to check the banned list for banned items every week.")
			return
		}

		err = insertSuggestion(db, ctx.Msg.Author.ID, char, weap, crown, skin)
		if err != nil {
			ctx.Reply("Failed to save suggestion.")
		}
	}
}

func isBanned(db *sql.DB, char, weap, crown string) (bool, error) {
	rows, err := db.Query(
		"SELECT * FROM weekly_banned WHERE chars = ? OR weaps = ? OR crowns = ?;",
		char,
		weap,
		crown,
	)

	if err != nil {
		log.Println("checkBanned: failed to retrieve rows:", err)
		return false, err
	}

	defer rows.Close()

	// Should maybe cache result
	var bannedChar, bannedCrown, bannedWeap string
	for rows.Next() {
		rows.Scan(&bannedChar, &bannedCrown, &bannedWeap)
		if bannedChar != "" || bannedCrown != "" || bannedWeap != "" {
			return true, nil
		}
	}

	return false, nil
}

func getUserSuggestionCount(db *sql.DB, id string) int {
	rows, err := db.Query("SELECT count FROM user_suggestions WHERE id = ?;", id)
	if err != nil {
		log.Println("getUserSuggestionCount: failed to get rows:", err)
		return -1
	}

	defer rows.Close()

	var count int
	for rows.Next() {
		rows.Scan(&count)
	}

	return count
}

func insertSuggestion(db *sql.DB, uid, char, weap, crown string, skin bool) error {
	// order is uid, char, skin, weap, crown
	stmt, err := db.Prepare("INSERT INTO weekly_suggestions VALUES(?, ?, ?, ?);")
	if err != nil {
		log.Println("inserSuggestion: failed to prepare stmt:", err)
		return err
	}

	defer stmt.Close()

	useSkin := 0
	if skin {
		useSkin = 1
	}

	_, err = stmt.Exec(uid, char, useSkin, weap, crown)
	if err != nil {
		log.Println("inserSuggestion: failed to insert into table stmt:", err)
		return err
	}

	return updateSuggestionCount(db, uid)
}

func updateSuggestionCount(db *sql.DB, uid string) (err error) {
	var (
		tx   *sql.Tx
		stmt *sql.Stmt
	)

	tx, err = db.Begin()
	if err != nil {
		log.Println("updateSuggestionCount: failed to begin Tx:", err)
		return
	}

	stmt, err = tx.Prepare("INSERT INTO user_suggestions(id, count) VALUES(?, ?) ON CONFLICT(id) DO UPDATE SET count = count + 1;")
	if err != nil {
		log.Println("updateSuggestionCount: failed to prepare stmt:", err)
		return tx.Rollback()
	}

	defer stmt.Close()

	_, err = stmt.Exec(uid, 1)
	if err != nil {
		log.Println("updateSuggestionCount: failed to exec stmt:", err)
		return tx.Rollback()
	}
	return tx.Commit()
}
