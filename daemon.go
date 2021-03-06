/*
goircd -- minimalistic simple Internet Relay Chat (IRC) server
Copyright (C) 2014 Sergey Matveev <stargrave@stargrave.org>
Copyright (C) 2022-	Terminal Wars Contributors

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU General Public License as published by
the Free Software Foundation, either version 3 of the License, or
(at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
GNU General Public License for more details.

You should have received a copy of the GNU General Public License
along with this program.  If not, see <http://www.gnu.org/licenses/>.
*/
package main

import (
	"fmt"
	"io/ioutil"
	"log"
	"net"
	"regexp"
	"sort"
	"strings"
	"time"
)

const (
	PING_TIMEOUT    = time.Second * 180 // Max time deadline for client's unresponsiveness
	PING_THRESHOLD  = time.Second * 90  // Max idle client's time before PING are sent
	ALIVENESS_CHECK = time.Second * 10  // Client's aliveness check period
)

var (
	RE_NICKNAME = regexp.MustCompile("^[a-zA-Z0-9-_]{1,9}$")
)

type Daemon struct {
	Verbose              bool
	hostname             string
	motd                 string
	clients              map[*Client]bool
	rooms                map[string]*Room
	room_sinks           map[*Room]chan ClientEvent
	last_aliveness_check time.Time
	log_sink             chan<- LogEvent
	state_sink           chan<- StateEvent
}

func NewDaemon(hostname, motd string, log_sink chan<- LogEvent, state_sink chan<- StateEvent) *Daemon {
	daemon := Daemon{hostname: hostname, motd: motd}
	daemon.clients = make(map[*Client]bool)
	daemon.rooms = make(map[string]*Room)
	daemon.room_sinks = make(map[*Room]chan ClientEvent)
	daemon.log_sink = log_sink
	daemon.state_sink = state_sink
	return &daemon
}

func (daemon *Daemon) SendLusers(client *Client) {
	lusers := 0
	for client := range daemon.clients {
		if client.registered {
			lusers++
		}
	}
	client.ReplyNicknamed(fmt.Sprintf("There are %d users and 0 invisible on 1 servers", lusers))
}

func (daemon *Daemon) SendMotd(client *Client) {
	if len(daemon.motd) == 0 {
		client.ReplyNicknamed("MOTD File is missing")
		return
	}

	motd, err := ioutil.ReadFile(daemon.motd)
	if err != nil {
		log.Printf("Can not read motd file %s: %v", daemon.motd, err)
		client.ReplyNicknamed("Error reading MOTD File")
		return
	}

	client.ReplyNicknamed("- "+daemon.hostname+" Message of the day -")
	for _, s := range strings.Split(strings.Trim(string(motd), "\n"), "\n") {
		client.ReplyNicknamed("- "+string(s))
	}
	client.ReplyNicknamed("End of /MOTD command")
}

func (daemon *Daemon) SendWhois(client *Client, nicknames []string) {
	for _, nickname := range nicknames {
		nickname = strings.ToLower(nickname)
		found := false
		for c := range daemon.clients {
			if strings.ToLower(c.nickname) != nickname {
				continue
			}
			found = true
			h := c.conn.RemoteAddr().String()
			h, _, err := net.SplitHostPort(h)
			if err != nil {
				log.Printf("Can't parse RemoteAddr %q: %v", h, err)
				h = "Unknown"
			}
			client.ReplyNicknamed(c.nickname, c.username, h, "*", c.realname)
			client.ReplyNicknamed(c.nickname, daemon.hostname, daemon.hostname)
			subscriptions := []string{}
			for _, room := range daemon.rooms {
				for subscriber := range room.members {
					if subscriber.nickname == nickname {
						subscriptions = append(subscriptions, room.name)
					}
				}
			}
			sort.Strings(subscriptions)
			client.ReplyNicknamed(c.nickname, strings.Join(subscriptions, " "))
			client.ReplyNicknamed(c.nickname, "End of /WHOIS list")
		}
		if !found {
			client.ReplyNoNickChan(nickname)
		}
	}
}

func (daemon *Daemon) SendList(client *Client, cols []string) {
	var rooms []string
	if (len(cols) > 1) && (cols[1] != "") {
		rooms = strings.Split(strings.Split(cols[1], " ")[0], ",")
	} else {
		rooms = []string{}
		for room := range daemon.rooms {
			rooms = append(rooms, room)
		}
	}
	sort.Strings(rooms)
	for _, room := range rooms {
		r, found := daemon.rooms[room]
		if found {
			client.ReplyNicknamed(room, fmt.Sprintf("%d", len(r.members)), r.topic)
		}
	}
	client.ReplyNicknamed("End of /LIST")
}

// Unregistered client workflow processor. Unregistered client:
// * is not PINGed
// * only QUIT, NICK and USER commands are processed
// * other commands are quietly ignored
// When client finishes NICK/USER workflow, then MOTD and LUSERS are send to him.
func (daemon *Daemon) ClientRegister(client *Client, command string, cols []string) {
	switch command {
	case "NICK":
		if len(cols) == 1 || len(cols[1]) < 1 {
			client.ReplyParts("No nickname given")
			return
		}
		nickname := cols[1]
		// something, somewhere, puts a colon before registered names. remove that.
		nickname = strings.Replace(nickname,":","",1)
		for client_ := range daemon.clients {
			if client_.nickname == nickname {
				client.ReplyParts("*", nickname, "Nickname is already in use")
				return
			}
		}
		found := ""
		for _, v := range nickname {
			if(!RE_NICKNAME.MatchString(string(v))) {
				found += string(v)+", "
			}
		}
		if len(found) >= 1 {
			client.ReplyParts("*", cols[1], "Erroneous nickname; contains "+found)
			return
		}
		client.nickname = nickname
	case "USER":
		if len(cols) == 1 {
			client.ReplyNotEnoughParameters("USER")
			return
		}
		args := strings.SplitN(cols[1], " ", 4)
		if len(args) < 4 {
			client.ReplyNotEnoughParameters("USER")
			return
		}
		client.username = args[0]
		client.realname = strings.TrimLeft(args[3], ":")
	}
	if client.nickname != "*" && client.username != "" {
		client.registered = true
		go daemon.HandlerJoin(client, "#TESTING")
		/*client.ReplyNicknamed("Hi, welcome to IRC")
		client.ReplyNicknamed("Your host is "+daemon.hostname+", running goircd")
		client.ReplyNicknamed("This server was created sometime")
		client.ReplyNicknamed(daemon.hostname+" goircd o o")
		daemon.SendLusers(client)
		daemon.SendMotd(client)*/
	}
}

// Register new room in Daemon. Create an object, events sink, save pointers
// to corresponding daemon's places and start room's processor goroutine.
func (daemon *Daemon) RoomRegister(name string) (*Room, chan<- ClientEvent) {
	room_new := NewRoom(daemon.hostname, name, daemon.log_sink, daemon.state_sink)
	room_new.Verbose = daemon.Verbose
	room_sink := make(chan ClientEvent)
	daemon.rooms[name] = room_new
	daemon.room_sinks[room_new] = room_sink
	go room_new.Processor(room_sink)
	return room_new, room_sink
}

func (daemon *Daemon) RoomGet(name string) (*Room, chan<- ClientEvent) {
	room_new := daemon.rooms[name]
	room_sink := daemon.room_sinks[room_new]
	return room_new, room_sink
}

func (daemon *Daemon) HandlerJoin(client *Client, cmd string) {
	// If the client is in a room already, part them from it.
	if(client.inRoom != "") {
		if(client.inRoom == cmd) {
			client.ReplyAlreadyInChannel(cmd)
			return
		} else {
			daemon.HandlerPart(client,client.inRoom)
		}
	}
	args := strings.Split(cmd, " ")
	rooms := strings.Split(args[0], ",")
	var keys []string
	if len(args) > 1 {
		keys = strings.Split(args[1], ",")
	} else {
		keys = []string{}
	}
	for n, room := range rooms {
		if !RoomNameValid(room) {
			client.ReplyNoChannel(room)
			continue
		}
		var key string
		if (n < len(keys)) && (keys[n] != "") {
			key = keys[n]
		} else {
			key = ""
		}
		denied := false
		joined := false
		for room_existing, room_sink := range daemon.room_sinks {
			if room == room_existing.name {
				if (room_existing.key != "") && (room_existing.key != key) {
					denied = true
				} else {
					room_sink <- ClientEvent{client, EVENT_NEW, ""}
					joined = true
				}
				break
			}
		}
		if denied {
			client.ReplyNicknamed(room, "Cannot join channel (+k) - bad key")
		}
		var room_new *Room
		var room_sink chan<- ClientEvent 
		if !(denied || joined) {
			room_new, room_sink = daemon.RoomRegister(room)
		} else {
			room_new, room_sink = daemon.RoomGet(room)
		}
		if key != "" {
			room_new.key = key
			room_new.StateSave()
		}
		client.inRoom = strings.ToUpper(cmd)
		room_sink <- ClientEvent{client, EVENT_NEW, ""}
	}
}

func (daemon *Daemon) HandlerPart(client *Client, cmd string) {
	for _, room := range strings.Split(cmd, ",") {
		r, found := daemon.rooms[room]
		if !found {
			client.ReplyNoChannel(room)
			continue
		}
		daemon.room_sinks[r] <- ClientEvent{client, EVENT_DEL, ""}
	}
}

func (daemon *Daemon) HandlerMsg(client *Client, cmd string) {

}

func (daemon *Daemon) Processor(events <-chan ClientEvent) {
	for event := range events {
		// Check for clients aliveness
		now := time.Now()
		if daemon.last_aliveness_check.Add(ALIVENESS_CHECK).Before(now) {
			for c := range daemon.clients {
				if c.timestamp.Add(PING_TIMEOUT).Before(now) {
					log.Println(c, "ping timeout")
					c.conn.Close()
					continue
				}
				if !c.ping_sent && c.timestamp.Add(PING_THRESHOLD).Before(now) {
					if c.registered {
						c.Msg("PING :" + daemon.hostname)
						c.ping_sent = true
					} else {
						log.Println(c, "ping timeout")
						c.conn.Close()
					}
				}
			}
			daemon.last_aliveness_check = now
		}

		client := event.client
		// placeholders
		replacer := strings.NewReplacer(
			"%%ROOM%%", client.inRoom,
		)
		switch event.event_type {
		case EVENT_NEW:
			daemon.clients[client] = true
		case EVENT_DEL:
			delete(daemon.clients, client)
			for _, room_sink := range daemon.room_sinks {
				room_sink <- event
			}
		case EVENT_MSG:
			// Split whatever message we got.
			cols_ := strings.SplitN(event.text, " ", 2)
			cols := make([]string,len(cols_))
			// Replace all the placeholder text in the array with the respecive thing
			for i, v := range cols_ {
				cols[i] = replacer.Replace(v) 
			}
			command := strings.ToUpper(cols[0])
			if daemon.Verbose {
				log.Println(client, "command", command)
			}
			if command == "QUIT" {
				delete(daemon.clients, client)
				client.conn.Close()
				continue
			}
			if !client.registered {
				go daemon.ClientRegister(client, command, cols)
				continue
			}
			switch command {
			case "AWAY", "PONG":
				continue
			case "JOIN":
				if len(cols) == 1 || len(cols[1]) < 1 {
					client.ReplyNotEnoughParameters("JOIN")
					continue
				}
				go daemon.HandlerJoin(client, cols[1])
			case "LIST":
				daemon.SendList(client, cols)
			case "LUSERS":
				go daemon.SendLusers(client)
			case "MODE":
				if len(cols) == 1 || len(cols[1]) < 1 {
					client.ReplyNotEnoughParameters("MODE")
					continue
				}
				cols = strings.SplitN(cols[1], " ", 2)
				if cols[0] == client.username {
					if len(cols) == 1 {
						client.Msg("221 " + client.nickname + " +")
					} else {
						client.ReplyNicknamed("Unknown MODE flag")
					}
					continue
				}
				room := cols[0]
				r, found := daemon.rooms[room]
				if !found {
					client.ReplyNoChannel(room)
					continue
				}
				if len(cols) == 1 {
					daemon.room_sinks[r] <- ClientEvent{client, EVENT_MODE, ""}
				} else {
					daemon.room_sinks[r] <- ClientEvent{client, EVENT_MODE, cols[1]}
				}
			case "MOTD":
				go daemon.SendMotd(client)
			case "PART":
				if len(cols) == 1 || len(cols[1]) < 1 {
					client.ReplyNotEnoughParameters("PART")
					continue
				}
				go daemon.HandlerPart(client, cols[1])
			case "PING":
				if len(cols) == 1 {
					client.ReplyNicknamed("No origin specified")
					continue
				}
				client.Reply(fmt.Sprintf("PONG %s :%s", daemon.hostname, cols[1]))
			case "NOTICE", "PRIVMSG":
				if len(cols) == 1 {
					client.ReplyNicknamed("No recipient given ("+command+")")
					continue
				}
				cols = strings.SplitN(cols[1], " ", 2)
				if len(cols) == 1 {
					client.ReplyNicknamed("No text to send")
					continue
				}
				msg := ""
				target := strings.ToLower(cols[0])
				sent := false
				for c := range daemon.clients {
					if c.nickname == target && c.inRoom == client.inRoom {
						msg = fmt.Sprintf("%s >> %s: %s", c.nickname, target, cols[1])
						c.Msg(msg)
						break
					}
					sent = true
				}
				if(!sent) {
					client.ReplyNicknamed("No recipients found in "+client.inRoom+" with that name.")
				}
			case "MSG":
				if len(cols) == 1 {
					client.ReplyNicknamed("No channel given (MSG)")
					continue
				}
				cols = strings.SplitN(cols[1], " ", 2)
				if len(cols) == 1 {
					client.ReplyNicknamed("No text to send")
					continue
				}
				target := strings.ToUpper(cols[0])
				r, found := daemon.rooms[target]
				if !found {
					client.ReplyNoNickChan(target)
					continue
				}
				daemon.room_sinks[r] <- ClientEvent{client, EVENT_MSG, "<"+client.nickname+"> "+cols[1]}
				client.Reply("<"+client.nickname+"> "+cols[1])
			case "TOPIC":
				if len(cols) == 1 {
					client.ReplyNotEnoughParameters("TOPIC")
					continue
				}
				cols = strings.SplitN(cols[1], " ", 2)
				r, found := daemon.rooms[cols[0]]
				if !found {
					client.ReplyNoChannel(cols[0])
					continue
				}
				var change string
				if len(cols) > 1 {
					change = cols[1]
				} else {
					change = ""
				}
				daemon.room_sinks[r] <- ClientEvent{client, EVENT_TOPIC, change}
			case "WHO":
				if len(cols) == 1 || len(cols[1]) < 1 {
					client.ReplyNotEnoughParameters("WHO")
					continue
				}
				room := strings.Split(cols[1], " ")[0]
				r, found := daemon.rooms[room]
				if !found {
					client.ReplyNoChannel(room)
					continue
				}
				daemon.room_sinks[r] <- ClientEvent{client, EVENT_WHO, ""}
			case "WHOIS":
				if len(cols) == 1 || len(cols[1]) < 1 {
					client.ReplyNotEnoughParameters("WHOIS")
					continue
				}
				cols := strings.Split(cols[1], " ")
				nicknames := strings.Split(cols[len(cols)-1], ",")
				go daemon.SendWhois(client, nicknames)
			default:
				client.ReplyNicknamed(command, "Unknown command")
			}
		}
	}
}
