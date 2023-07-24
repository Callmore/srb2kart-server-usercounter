package main

import (
	"bufio"
	"database/sql"
	"flag"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"time"

	"srb2kart-server-usercounter/srb2kart"

	_ "github.com/mattn/go-sqlite3"
)

const baseURL = "https://ms.kartkrew.org/ms/api/games/SRB2Kart"

type ServerInfo struct {
	Address string
	Port    string
}

func (info ServerInfo) String() string {
	return fmt.Sprintf("%s:%s", info.Address, info.Port)
}

func getNewestVersion() int {
	path, err := url.JoinPath(baseURL, "version")
	if err != nil {
		panic(err)
	}

	resp, err := http.Get(path + "?v=2")
	if err != nil {
		panic(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		panic(fmt.Errorf("status code %d", resp.StatusCode))
	}

	var version int
	_, err = fmt.Fscanf(resp.Body, "%d", &version)
	if err != nil {
		panic(err)
	}

	return version
}

func getServerList(version int) []ServerInfo {
	path, err := url.JoinPath(baseURL, strconv.Itoa(version), "servers")
	if err != nil {
		panic(err)
	}

	resp, err := http.Get(path + "?v=2")
	if err != nil {
		panic(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		panic(fmt.Errorf("status code %d", resp.StatusCode))
	}

	servers := []ServerInfo{}
	scanner := bufio.NewScanner(resp.Body)
	for scanner.Scan() {
		var server ServerInfo
		_, err = fmt.Sscanf(scanner.Text(), "%s %s", &server.Address, &server.Port)
		if err != nil {
			panic(err)
		}
		servers = append(servers, server)
	}
	return servers
}

type ChannelResult[T any] struct {
	Ok    bool
	Value T
}

var databasePath = flag.String("o", "", "Sets the location of the database that will be written to.")

func main() {
	flag.Parse()
	if *databasePath == "" {
		flag.Usage()
		return
	}

	serverIn := make(chan string)
	serverOut := make(chan ChannelResult[srb2kart.ServerInfo])
	serverCount := make(chan int)

	timestamp := time.Now().Unix()

	go func() {
		servers := getServerList(getNewestVersion())
		serverCount <- len(servers)
		for _, v := range servers {
			serverIn <- v.String()
		}
		close(serverIn)
	}()

	for i := 0; i < 16; i++ {
		go func() {
			for {
				server, ok := <-serverIn
				if !ok {
					break
				}

				result, err := srb2kart.GetServerInfo(server)
				if err != nil {
					fmt.Printf("Error: %s\n", err)
					serverOut <- ChannelResult[srb2kart.ServerInfo]{
						Ok: false,
					}
					continue
				}

				serverOut <- ChannelResult[srb2kart.ServerInfo]{
					Ok:    true,
					Value: result,
				}
			}
		}()
	}

	// Setup SQLite Database
	db, err := sql.Open("sqlite3", os.Getenv("DATABASE_PATH"))
	if err != nil {
		panic(err)
	}

	_, err = db.Exec("CREATE TABLE IF NOT EXISTS servers (timestamp INTEGER, name BLOB, players INTEGER, maxPlayers INTEGER);")
	if err != nil {
		panic(err)
	}

	count := <-serverCount
	for i := 0; i < count; i++ {
		infoResult := <-serverOut
		if !infoResult.Ok {
			continue
		}

		info := infoResult.Value

		db.Exec("INSERT INTO servers VALUES (?, ?, ?, ?)", timestamp, info.ServerNameRaw, info.Players, info.MaxPlayers)
	}
}
