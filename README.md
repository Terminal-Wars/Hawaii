# Hawaii

A basic IRC\* server forked from [ThomasHabets/goircd](https://github.com/ThomasHabets/goircd) with support for the battle/games features that Terminal Wars needs. [Named after Hawaii Part II](https://www.youtube.com/watch?v=NbtsZJXnzFY).

\**IRC was used as a building block for this but this is by no means concerned with being IRC complient, especially because this isn't meant for a regular IRC client.*

*Notable changes and additions:*

* Altered, more simplified user messages (for the most part, only the username, and message is shown, as well as avatars when those are added)
* New MSG command specifically for messaging channels, that has a different syntax from what PRIVMSG gives.
* Error codes are removed. 