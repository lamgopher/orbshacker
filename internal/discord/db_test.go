package discord

import (
	"reflect"
	"testing"
)

func testDB() *DB {
	return &DB{Games: []Game{
		{ID: "1", Name: "PUBG: BATTLEGROUNDS", Aliases: []string{"PUBG"}},
		{ID: "2", Name: "PUBG Lite"},
		{ID: "3", Name: "League of Legends", Aliases: []string{"LoL"}},
		{ID: "1", Name: "PUBG: BATTLEGROUNDS duplicate"},
	}}
}

func TestSearchExactFirst(t *testing.T) {
	got := ids(testDB().Search("pubg", 0))
	want := []string{"1", "2"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Search() = %v, want %v", got, want)
	}
}

func TestSearchByAlias(t *testing.T) {
	got := ids(testDB().Search("lol", 0))
	if !reflect.DeepEqual(got, []string{"3"}) {
		t.Fatalf("Search() = %v", got)
	}
}

func TestSearchLimitAndEmpty(t *testing.T) {
	if got := testDB().Search("pubg", 1); len(got) != 1 {
		t.Fatalf("limit not applied: %v", ids(got))
	}
	if got := testDB().Search("  ", 0); got != nil {
		t.Fatalf("empty query should return nil, got %v", ids(got))
	}
}

func TestExecutables(t *testing.T) {
	g := Game{Executables: []Executable{
		{OS: "darwin", Name: "game.app"},
		{OS: "win32", Name: "launcher.exe"},
		{OS: "win32", Name: `>bin\win64\Game.exe`},
		{OS: "win32", Name: "bin/win64/Game.exe"},
		{OS: "win32", Name: "Game_BE.exe"},
	}}

	if got := PrimaryExecutable(g); got != "bin/win64/Game.exe" {
		t.Fatalf("PrimaryExecutable() = %q", got)
	}
	want := []string{"bin/win64/Game.exe", "launcher.exe", "Game_BE.exe"}
	if got := AllExecutables(g); !reflect.DeepEqual(got, want) {
		t.Fatalf("AllExecutables() = %v, want %v", got, want)
	}
}

func TestPrimaryExecutableNone(t *testing.T) {
	g := Game{Executables: []Executable{{OS: "win32", Name: "uninstall.exe"}}}
	if got := PrimaryExecutable(g); got != "" {
		t.Fatalf("PrimaryExecutable() = %q, want empty", got)
	}
}

func TestSteamAppID(t *testing.T) {
	g := Game{ThirdPartySKUs: []SKU{{Distributor: "xbox", ID: "9NG0FFHT5MCB"}, {Distributor: "steam", ID: "366320"}}}
	if id, ok := SteamAppID(g); !ok || id != 366320 {
		t.Fatalf("SteamAppID() = %d, %v", id, ok)
	}
	if _, ok := SteamAppID(Game{ThirdPartySKUs: []SKU{{Distributor: "epic", ID: "abc"}}}); ok {
		t.Fatal("SteamAppID() should be false without a steam SKU")
	}
}

func ids(games []Game) []string {
	var r []string
	for _, g := range games {
		r = append(r, g.ID)
	}
	return r
}
