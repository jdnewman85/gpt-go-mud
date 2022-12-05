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

// player represents a player in the MUD.
type player struct {
	health int
	mana   int
	x      int
	y      int
}

// connection represents a connection to the MUD.
type connection struct {
	conn   net.Conn
	name   string
	state  int
	output *bufio.Writer
	player *player
}

// newConnection creates a new connection.
func newConnection(conn net.Conn) *connection {
	return &connection{
		conn:   conn,
		state:  stateLogin,
		output: bufio.NewWriter(conn),
		player: nil,
	}
}

// mud represents the MUD server.
type mud struct {
	listener net.Listener
	conns    map[string]*connection
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
	c := &connection{
		conn:   conn,
		state:  stateLogin,
		output: bufio.NewWriter(conn),
	}
	m.conns[conn.RemoteAddr().String()] = c
	c.write("Welcome to the MUD!\n\nEnter your name: ")
	return c, nil
}

// handlePlaying processes commands from the connection in the playing state.
func (m *mud) handlePlaying(c *connection, cmd string, args []string) {
	switch cmd {
	case "who":
		m.who(c)
	case "say":
		m.say(c, args)
	case "quit":
		m.quit(c)
	default:
		c.write("Unknown command: " + cmd + "\n")
	}
	c.write(fmt.Sprintf("\n%s - Health: %d Mana: %d > ", c.name, c.player.health, c.player.mana))
}

// handleLogin processes the name entered by the connection.
func (m *mud) handleLogin(c *connection, name string) {
	if len(name) < 3 {
		c.write("Name must be at least 3 characters long.\n\nEnter your name: ")
		return
	}
	if _, ok := m.conns[name]; ok {
		c.write("Name is already taken.\n\nEnter your name: ")
		return
	}
	delete(m.conns, c.conn.RemoteAddr().String())
	c.name = name
	m.conns[name] = c
	c.write("Enter your password: ")
	c.state = statePassword
}

// handlePassword processes commands from the connection in the password state.
func (m *mud) handlePassword(c *connection, cmd string) {
	if len(cmd) <= 4 || !strings.ContainsAny(cmd, "0123456789") {
		c.write("Password must be at least 5 characters long and contain a number.\n")
		c.write("Enter your password: ")
		return
	}
	c.state = statePlaying
	c.player = &player{
		health: 100,
		mana:   100,
		x:      0,
		y:      0,
	}
	c.write("Welcome to the MUD, " + c.name + "!\n")
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

func main() {
	m := newMud()
	if err := m.listen("localhost:8000"); err != nil {
		fmt.Println(err)
		return
	}
	fmt.Println("Listening on localhost:8000")
	for {
		c, err := m.acceptConnection()
		if err != nil {
			fmt.Println(err)
			break
		}
		go m.handleConnection(c)
	}
}

// write writes the given string to the connection's output buffer.
func (c *connection) write(s string) {
	if _, err := fmt.Fprintf(c.output, s); err != nil {
		fmt.Println(err)
	}
	if err := c.output.Flush(); err != nil {
		fmt.Println(err)
	}
}

// flush flushes the connection's output buffer to the connection.
func (c *connection) flush() {
	if err := c.output.Flush(); err != nil {
		fmt.Println(err)
	}
}

// broadcast sends the given string to all connections in the playing state.
func (m *mud) broadcast(s string) {
	for _, c := range m.conns {
		if c.state == statePlaying {
			c.write(s)
		}
	}
}

// who displays the list of connections in the playing state to the given connection.
func (m *mud) who(c *connection) {
	c.write("Players:\n")
	for _, conn := range m.conns {
		if conn.state == statePlaying {
			c.write("  " + conn.name + "\n")
		}
	}
}

// say broadcasts the given message to all connections in the playing state.
func (m *mud) say(c *connection, args []string) {
	if len(args) == 0 {
		c.write("Say what?\n")
		return
	}
	m.broadcast(c.name + " says: " + strings.Join(args, " ") + "\n")
}

// quit disconnects the given connection with a bye message.
func (m *mud) quit(c *connection) {
	c.write("Bye!\n")
	c.state = stateDead
}
