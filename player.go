package main

import (
)

type Player struct { 	// A player character that somebody can control in battle.
	name 				string			// Name of the player character
	owner 				string 			// The nickname one must have to control the player.
	owner_conn 			*Client			// The client that currently owns them; nil if offline.

	aggressive 			bool			// Whether they're an NPC or not
	hp 					uint16			// Their HP.

	passives  			[]*Move			// Moves that are activated when the player enters battle
	optionalPassives 	[]*Move 		// Moves that have lasting effects until the user perishes.
	actives 			[]*Move			// Moves that the player can active themselves.
}

type Move struct { 		// A move that the player can either have used at the start of a battle, or used on their turn.  
	name 				string 			// Name of the move, internally.
	prettyName 			string			// Name of the move, with capitlization.
	bio 	 			string			// Description of the move.
	instruction 		string			// How the move plays out.
	cooldown  			int16 			// How many turns a player can use this move
	limit	  			int16 			// How many times this move can be used.

	on_activate 		[]Command 		// Commands executed once the move is used.
	on_deactivate 		[]Command		// Commands executed after the move is used.
}

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

type Command struct {
	parts 				[]Value
}

type Condition struct {
	part 				[]Value
}

type Value interface { // All the types that a part of a command/condition can be.
	String()  			string
	Int() 				int
	UInt() 				uint
	Float32() 			float32
	Float64()  			float64
}

