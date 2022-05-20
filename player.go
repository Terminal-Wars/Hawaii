package main

import (
	//"fmt"
	"os"
	"encoding/json"
	"errors"
	"strconv"
	"strings"
)


type Player struct {	// A player character that somebody can control in battle.
	name 				string			// Name of the player character
	owner 				string 			// The nickname one must have to control the player.
	owner_conn 			*Client			// The client that currently owns them; nil if offline.
										// (also nil if its a Player that globally be used by all)

	aggressive 			bool			// Whether they're an NPC or not
	hp 					int			// Their HP.

	passives  			*[]Move			// Moves that are activated when the player enters battle
	optionalPassives 	*[]Move 		// Moves that have lasting effects until the user perishes.
	actives 			*[]Move			// Moves that the player can active themselves.
}

var recognizedPlayers map[string]Player

type Move struct {		// A move that the player can either have used at the start of a battle, or used on their turn.  
	name 				string 			// Name of the move, internally.
	prettyName 			string			// Name of the move, with capitlization.
	bio 	 			string			// Description of the move.
	instruction 		string			// How the move plays out.
	cooldown  			int16 			// How many turns a player can use this move
	limit	  			int16 			// How many times this move can be used.

	on_activate 		[]Command 		// Commands executed once the move is used.
	on_deactivate 		[]Command		// Commands executed after the move is used.
}

var recognizedMoveTables [][]Move

type Group struct {		// A group of moves, usually visible when a passive is active.
	name 				string 			// Group name
	prettyName 			string			// Group name with capitalization
	bio 	 			string 			// Group description

	prereq 				[]Condition 	// A set of conditions that dictate when this group is visible. 
	prereq_string 		string 			// A description of that pre-requisite.

	cooldown  			int16 			// Cooldown on any of the moves in this group.
	limit 				int16 			// How many times any of the moves in this group can be used.

	moves		 		[]Move			// The moves in this group
}

type Command struct {	// A command that a move activates in battle
	name  				string 			// Name of it
	args 				[]string 		// The arguments to pass to it
	on_succeed 			[]Command 		// The commands to execute when it succeeds
	on_fail 			[]Command 		// The commands to execute when it fails
	info 				Player 			// A special use case for the "bot" command which summons another player struct
}

type Condition struct {	
	name  				string
	meets 				string
}

func init() {
	recognizedPlayers = make(map[string]Player)
}

func LoadPlayer(filename, playername string) (error) {
	// Get the filename
	file, err := os.ReadFile(filename)
	if(err != nil) {
		return errors.New("Couldn't read player file: \n"+err.Error())
	}
	// Create a new object from the file
	var jsonFile map[string]json.RawMessage
	err = json.Unmarshal(file, &jsonFile)
	if err != nil {
		return errors.New("Could not unmarshal the player file: \n"+err.Error())
	}

	player, err := NewPlayer(jsonFile)
	if(err != nil) {
		return errors.New("Couldn't load the player file into a player object: \n"+err.Error())
	}

	recognizedPlayers[playername] = player

	return nil
}

func NewPlayer(o map[string]json.RawMessage) (Player, error) {
	// All the values we want

	var name, owner string
	var aggressive bool
	var hp int
	var passives, optionalPassives, actives *[]Move

	var err error

	// The first three values are actually required, an error should be thrown if they're not present.
	if(o["character"] != nil) {
		name = string(o["character"])
	} else {
		return Player{}, errors.New("No name was given for this player.")
	}
	if(o["owner"] != nil) {
		owner = string(o["owner"])
	} else {
		return Player{}, errors.New("No owner nickname was given for this player.")
	}
	if(o["aggressive"] != nil) {
		aggressive_ := string(o["aggressive"])
		switch(aggressive_) {
			case "\"false\"", "\"no\"":
				aggressive = false
			case "\"true\"", "\"yes\"":
				aggressive = true
			default:
				return Player{}, errors.New("The player's aggressive value wasn't true or false, it was "+aggressive_)
		}
	} else {
		return Player{}, errors.New("This player wasn't specified to be aggressive or not")
	}

	if(o["hp"] != nil) {
		hpstr := strings.Replace(string(o["hp"]),"\"","",2)
		hp, err = strconv.Atoi(hpstr)
		if(err != nil) {
			return Player{}, errors.New("Invalid HP given: "+hpstr)
		}
	} else {
		hp = 1
	}

	// Load the moves
	if(o["actives"] != nil) {
		actives, err = ParseMoveTable(o["actives"])
		if err != nil {return Player{}, err}
	}
	if(o["optional_passives"] != nil) {
		optionalPassives, err = ParseMoveTable(o["optional_passives"])
		if err != nil {return Player{}, err}
	}
	if(o["passives"] != nil) {
		passives, err = ParseMoveTable(o["passives"])
		if err != nil {return Player{}, err}
	}


	player := Player{name,owner,nil,aggressive,hp,passives,optionalPassives,actives}
	return player, nil
}

func ParseMoveTable(table []byte) (*[]Move, error) {
	var moves []Move
	// Create a new object from the table
	var moveObj []interface{}
	err := json.Unmarshal(table, &moveObj)
	if err != nil {
		return nil, errors.New("Could not unmarshal the move table: \n"+err.Error())
	}
	// And create a new object for each of the moves
	for _, v := range moveObj {
		move, err := ParseMove(v)
		if(err != nil) {
			return nil, err
		}
		moves = append(moves, move)
	}
	if(len(moves) >= 1) {
		recognizedMoveTables = append(recognizedMoveTables, moves)
		return &moves, nil
	}
	return nil, nil
}

func ParseMove(move interface{}) (Move, error) {
	w, ok := move.(map[string]interface{})
	if(ok) {
		// Start getting the values to fill our move with
		var name, prettyname, bio, instruction string
		var cooldown, limit int
		var onActivate []Command
		var onDeactivate []Command
		var err error

		if(w["name"] != nil) {name = w["name"].(string)}
		if(w["prettyname"] != nil) {prettyname = w["prettyname"].(string)}
		if(w["bio"] != nil) {bio = w["bio"].(string)}
		if(w["instruction"] != nil) {instruction = w["instruction"].(string)}

		if(w["cooldown"] != nil) {
			cooldown, err = strconv.Atoi(w["cooldown"].(string))
			if(err != nil) {
				return Move{}, errors.New("Couldn't parse the cooldown for "+name+": \n"+err.Error())
			}
		}
		if(w["limit"] != nil) {
			limit, err = strconv.Atoi(w["limit"].(string))
			if(err != nil) {
				return Move{}, errors.New("Couldn't parse the limit for "+name+": \n"+err.Error())
			}
		}

		// Parse the commands on each of these moves.
		if(w["on_activate"] != nil) {
			onActivate, err = ParseCommandTable(w["on_activate"])
			if(err != nil) {
				return Move{}, errors.New("Couldn't parse on_activate for "+name+": \n"+err.Error())
			}
		}
		if(w["on_deactivate"] != nil) {
			onDeactivate, err = ParseCommandTable(w["on_deactivate"])
			if(err != nil) {
				return Move{}, errors.New("Couldn't parse on_deactivate for "+name+": \n"+err.Error())
			}
		}

		move := Move{
			name: name,
			prettyName: prettyname,
			bio: bio,
			instruction: instruction,
			cooldown: int16(cooldown),
			limit: int16(limit),
			on_activate: onActivate,
			on_deactivate: onDeactivate,
		}
		return move, nil
	}
	return Move{}, errors.New("Rare error where a move cannot be unwrapped into an array of strings")
}

func ParseCommandTable(table interface{}) ([]Command, error) {
	var commands []Command
	// Is it an array of commands?
	v, ok := table.([]interface{})
	// If so, parse each one.
	if(ok) {
		for _, w := range v {
			x, ok := w.(map[string]interface{})
			if(ok) {
				command, err := ParseCommand(x)
				if(err != nil) {
					if(x["name"] != nil) {
						var name string
						name = x["command"].(string)
						return nil, errors.New("Couldn't parse the "+name+" command: \n\n"+err.Error())
					} else {
						return nil, errors.New("Couldn't parse the an unnamed command: \n\n"+err.Error())
					}
				}
				commands = append(commands, command)
			} else {
				commands = append(commands, Command{})
			}
		}
	// Otherwise, we only need to parse one.
	} else {
		w, ok := table.(map[string]interface{})
		if(ok) {
			command, err := ParseCommand(w)
			if(err != nil) {
				if(w["name"] != nil) {
					var name string
					name = w["command"].(string)
					return nil, errors.New("Couldn't parse the "+name+" command: \n\n"+err.Error())
				} else {
					return nil, errors.New("Couldn't parse the an unnamed command: \n\n"+err.Error())
				}
			}
			commands = append(commands, command)
		} else {
			commands = append(commands, Command{})
		}
	}
	return commands, nil
}

func ParseCommand(s map[string]interface{}) (Command, error) {
	var name string
	var args []string
	var on_succeed, on_fail []Command
	var info Player //(the bot command currently isn't implemented)
	var err error
	if(s["command"] != nil) {
		name = s["command"].(string)
	}
	if(s["args"] != nil) {
		// args is actually an interface too, we have to convert the strings individually
		v, ok := s["args"].([]interface{})
		if(ok) {
			for _, w := range v {
				args = append(args,w.(string))
			}
		}
	}
	if(s["on_succeed"] != nil) {
		v, ok := s["on_succeed"].(map[string]interface{})
		if(ok) {
			on_succeed, err = ParseCommandTable(v)
			if(err != nil) {
				return Command{}, errors.New("Couldn't parse a "+name+" command's on_succeed: \n"+err.Error())
			}
		}
	}
	if(s["on_fail"] != nil) {
		v, ok := s["on_fail"].(map[string]interface{})
		if(ok) {
			on_fail, err = ParseCommandTable(v)
			if(err != nil) {
				return Command{}, errors.New("Couldn't parse a "+name+" command's on_fail: \n"+err.Error())
			}
		}
	}
	command := Command{
		name: name,
		args: args,
		on_succeed: on_succeed,
		on_fail: on_fail,
		info: info,
	}
	return command, nil
}