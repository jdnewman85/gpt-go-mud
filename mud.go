package main

import (
	"bufio"
	"fmt"
	"net"
	"strings"
)

const (
	// connection states
	stateLogin = iota
	statePassword
	statePlaying
	stateDead
)

// connection represents a connection to the MUD.
type connection struct {
	conn   net.Conn
	name   string
	state  int
	output *bufio.Writer
	player *player
}

// mud represents the MUD server.
type mud struct {
	listener net.Listener
	conns    map[string]*connection
}

// player represents a player in the MUD.
type player struct {
	health int
	mana   int
	x      int
	y      int
}

// newMud creates a new MUD server.
func newMud() *mud {
	return &mud{
		conns: make(map[string]*connection),
	}
}

// listen starts listening for connections on the given address.
func (m *mud) listen(addr string) error {
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		return err
	}
	m.listener = listener
	return nil
}

// acceptConnection accepts a new connection and adds it to the list of connections.
func (m *mud) acceptConnection() (*connection, error) {
	conn, err := m.listener.Accept()
	if err != nil {
		return nil, err
	}
	c := newConnection(conn)
	m.conns[conn.RemoteAddr().String()] = c
	c.write("Welcome to the MUD!\n\nEnter your name: ")
	return c, nil
}

// newConnection creates a new connection.
func newConnection(conn net.Conn) *connection {
	return &connection{
		conn:   conn,
		output: bufio.NewWriter(conn),
		state:  stateLogin,
	}
}

// handleConnection processes commands from the given connection.
func (m *mud) handleConnection(c *connection) {
	scanner := bufio.NewScanner(c.conn)
	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, " ", 2)
		cmd := parts[0]
		var args []string
		if len(parts) == 2 {
			args = strings.Split(parts[1], " ")
		}
		switch c.state {
		case stateLogin:
			m.handleLogin(c, cmd)
		case statePassword:
			m.handlePassword(c, cmd)
		case statePlaying:
			m.handlePlaying(c, cmd, args)
		case stateDead:
			// do nothing
		}
	}
}

// handleLogin processes login commands from the given connection.
func (m *mud) handleLogin(c *connection, cmd string) {
	c.name = cmd
	if len(c.name) < 3 {
		c.write("Name must be at least 3 characters.\nEnter your name: ")
		return
	}
	if _, ok := m.conns[c.name]; ok {
		c.write("Name is already in use.\nEnter your name: ")
		return
	}
	m.conns[c.name] = c
	delete(m.conns, c.conn.RemoteAddr().String())
	c.write("Enter your password: ")
	c.state = statePassword
}

// handlePassword processes password commands from the given connection.
func (m *mud) handlePassword(c *connection, cmd string) {
	if len(cmd) < 5 || !strings.ContainsAny(cmd, "0123456789") {
		c.write("Password must be at least 5 characters and contain a number.\nEnter your password: ")
		return
	}
	c.player = &player{}
	c.write(fmt.Sprintf("Welcome, %s!\n\n", c.name))
	c.state = statePlaying
}

// handlePlaying processes playing commands from the given connection.
func (m *mud) handlePlaying(c *connection, cmd string, args []string) {
	switch cmd {
	case "who":
		m.who(c)
	case "say":
		m.say(c, args)
	case "quit":
		m.quit(c)
	default:
		c.write("Unknown command.\n")
	}
	if c.state == statePlaying {
		c.write(fmt.Sprintf("%s: %d/%d > ", c.name, c.player.health, c.player.mana))
	}
}

// who displays the list of players in the playing state to the given connection.
func (m *mud) who(c *connection) {
	c.write("Connected players:\n")
	for _, conn := range m.conns {
		if conn.state == statePlaying {
			c.write(fmt.Sprintf("- %s\n", conn.name))
		}
	}
}

// say broadcasts the given message to all connections in the playing state.
func (m *mud) say(c *connection, args []string) {
	if len(args) == 0 {
		return
	}
	msg := strings.Join(args, " ")
	for _, conn := range m.conns {
		if conn.state == statePlaying {
			conn.write(fmt.Sprintf("%s says: %s\n", c.name, msg))
		}
	}
}

// quit disconnects the given connection.
func (m *mud) quit(c *connection) {
	c.write("Bye!\n")
	delete(m.conns, c.name)
	delete(m.conns, c.conn.RemoteAddr().String())
	c.conn.Close()
	c.state = stateDead
	for _, conn := range m.conns {
		if conn.state == statePlaying {
			conn.write(fmt.Sprintf("%s has quit.\n", c.name))
		}
	}
}

// write sends the given message to the connection.
func (c *connection) write(msg string) {
	c.output.WriteString(msg)
	c.output.Flush()
}

func main() {
	m := newMud()
	if err := m.listen("localhost:8080"); err != nil {
		panic(err)
	}
	for {
		c, err := m.acceptConnection()
		if err != nil {
			panic(err)
		}
		go m.handleConnection(c)
	}
}
