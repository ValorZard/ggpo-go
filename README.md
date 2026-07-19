# [![GGPO-Go LOGO](./ggpo_go_logo.png)](https://github.com/ikemen-engine/ggpo)
# GGPO-Go - GGPO Port Into Go
GGPO-Go is a port of the GGPO rollback netcode library into Go. Currently unfinished. 

## Usage 
General usage would be best explained by looking at the code in the example folder.

But to view the example, follow the following steps: 

- Clone the repository. 
- Travel to the example folder 
- enter:
    `go run . <local_port> <num_players>  <local|remote_ip:port> <local|remote_ip:port> <current_player>`

An example usage would be to open one command line input with the following command 
`go run . 7000 2 local 127.0.0.1:7001 1`
and another with this command 
`go run . 7001 2 127.0.0.1:7000 local 2`


If you want to have a spectator, make sure the spectator connect to the host FIRST before the host connects with player 2

player 1 (the "host" — note the spectator address as the last arg)
`go run . 7000 2 local 127.0.0.1:7001 1 127.0.0.1:7100`

spectator, listening on 7100, watching player 1
`go run . 7100 2 spectate 127.0.0.1:7000`

player 2
`go run . 7001 2 127.0.0.1:7000 local 2`

