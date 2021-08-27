package main

import (
	"context"
	"database/sql"
	"fmt"
	"github.com/uptrace/bun"
	"github.com/uptrace/bun/dbfixture"
	"github.com/uptrace/bun/dialect/sqlitedialect"
	"github.com/uptrace/bun/driver/sqliteshim"
	"github.com/uptrace/bun/extra/bundebug"
	"github.com/valyala/fasthttp"
	"gopkg.in/yaml.v2"
	"io/fs"
	"io/ioutil"
	"log"
	"os"
)

var (
	ctx, db            = loadDatabase(databaseName)
	databaseName       = "database.yaml"
	sampleDatabaseName = "sample-database.yaml"
)

// loadDatabase will load a new database from config/database.yaml or config/sample-database.yaml
func loadDatabase(file string) (context.Context, *bun.DB) {
	newCtx := context.Background()

	sqlite, err := sql.Open(sqliteshim.ShimName, "file::memory:?cache=shared")
	if err != nil {
		panic(err)
	}
	sqlite.SetMaxOpenConns(1)

	newDB := bun.NewDB(sqlite, sqlitedialect.New())
	newDB.AddQueryHook(bundebug.NewQueryHook(bundebug.WithVerbose()))

	// Register models for the fixture.
	newDB.RegisterModel((*User)(nil))

	// Create tables and load initial data.
	fixture := dbfixture.New(newDB, dbfixture.WithRecreateTables())
	if err := fixture.Load(newCtx, os.DirFS("config/"), file); err != nil {
		if file != sampleDatabaseName { // prevent recursive loop
			return loadDatabase(sampleDatabaseName)
		} else {
			panic(err)
		}
	}

	return newCtx, newDB
}

// GetUser will get a User with the provided state
func GetUser(state string) *User {
	// Select one user by their state key.
	user := new(User)
	errored := false
	fmt.Printf("user: %v", user)
	if err := db.NewSelect().Model(user).Where("state = ?", state).Scan(ctx); err != nil {
		errored = true
		log.Printf("- Failed to find user with 'state' '%s'", state)
	}

	if !errored {
		return user
	}
	return nil
}

// GetUserStatus will return the status code corresponding to the Cookie header validity
func GetUserStatus(ctx *fasthttp.RequestCtx) (int, string) {
	stateBytes := ctx.Request.Header.Cookie(cookieName)
	if len(stateBytes) == 0 { // No cookie header
		return fasthttp.StatusUnauthorized, "Not logged in!"
	}

	user := GetUser(string(stateBytes))
	if user == nil { // A user with the state returned by the cookie was not found in the database
		return fasthttp.StatusForbidden, "Invalid Cookie"
	}

	// else, we don't do anything if the user is found. The request will return 200
	return 200, "Logged In"
}

// InsertUser will insert a new User, or overwrite an existing user with a matching id
func InsertUser(user User) error {
	_, err := db.NewInsert().Model(&user).On("CONFLICT (id) DO UPDATE").
		Set("id = EXCLUDED.id").
		Set("state = EXCLUDED.state").
		Set("whitelisted = EXCLUDED.whitelisted").
		Exec(ctx)
	return err
}

// UpdateUserWhitelist will update the whitelisted status of a User matching id
func UpdateUserWhitelist(id int, whitelisted bool) error {
	user := new(User)
	user.Whitelisted = whitelisted
	_, err := db.NewUpdate().Model(user).Column("whitelisted").Where("id = ?", id).Exec(ctx)
	return err
}

// TODO: Is there really not a proper way to do this?
func saveDatabase() {
	users := make([]User, 0)
	if err := db.NewSelect().Model(&users).OrderExpr("id ASC").Scan(ctx); err != nil {
		panic(err)
	}

	if *debug {
		log.Printf("Users: %v", users)
	}

	formattedData := []fixtureData{{Model: "User", Rows: users}}
	data, err := yaml.Marshal(formattedData)
	if err != nil {
		panic(err)
	}

	err = ioutil.WriteFile("config/"+databaseName, data, fs.FileMode(0700))

	if err != nil {
		panic(err)
	}
}

func (u User) String() string {
	return fmt.Sprintf("User<%s, %v, %s, %v>", u.Name, u.ID, u.State, u.Whitelisted)
}

type User struct {
	Name        string
	ID          int
	State       string
	Whitelisted bool
}

type fixtureData struct {
	Model string `yaml:"model"`
	Rows  []User `yaml:"rows"`
}
