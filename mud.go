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

// handleLogin handles the stateLogin state.
func (m *mud) handleLogin(c *connection, args []string) {
	if len(args) != 2 {
		c.write("Invalid name.\nEnter your name: ")
		return
	}
	name := args[1]
	if len(name) < 3 {
		c.write("Name must be at least 3 characters.\nEnter your name: ")
		return
	}
	c.name = name
	c.write("Enter your password: ")
	c.state = statePassword
}

// handlePassword handles the statePassword state.
func (m *mud) handlePassword(c *connection, args []string) {
	if len(args) != 2 {
		c.write("Invalid password.\nEnter your password: ")
		return
	}
	password := args[1]
	if len(password) <= 4 || !strings.ContainsAny(password, "0123456789") {
		c.write("Password must be at least 4 characters and contain a number.\nEnter your password: ")
		return
	}
	c.write("\nWelcome to the game, " + c.name + "!\n\n")
	c.state = statePlaying
}

// handlePlaying handles the statePlaying state.
func (m *mud) handlePlaying(c *connection, args []string) {
	switch cmd := args[0]; cmd {
	case "who":
		c.write("Connections:\n")
		for _, conn := range m.conns {
			if conn.state == statePlaying {
				c.write("- " + conn.name + "\n")
			}
		}
	case "say":
		if len(args) < 2 {
			c.write("Usage: say <message>\n")
			return
		}
		message := strings.Join(args[1:], " ")
		for _, conn := range m.conns {
			if conn.state == statePlaying {
				conn.write(c.name + " says: " + message + "\n")
			}
		}
	}
}

// handleConnection processes commands from the given connection.
func (m *mud) handleConnection(c *connection) {
	input := bufio.NewScanner(c.conn)
	for input.Scan() {
		line := input.Text()
		args := strings.Fields(line)
		if len(args) == 0 {
			continue
		}
		command, args := args[0], args[1:]
		switch c.state {
		case stateLogin:
			if len(command) < 3 {
				c.write("Name must be at least 3 characters.\n\nEnter your name: ")
				break
			}
			c.name = command
			c.state = statePassword
			c.write("Enter your password: ")
		case statePassword:
			if len(command) < 5 || !strings.ContainsAny(command, "0123456789") {
				c.write("Password must be at least 5 characters and contain a number.\n\nEnter your password: ")
				break
			}
			c.state = statePlaying
			c.write("Welcome, " + c.name + "!\n")
			m.broadcast(c.name + " has entered the game.\n")
		case statePlaying:
			switch command {
			case "who":
				m.who(c)
			case "say":
				m.say(c, args)
			case "quit":
				m.quit(c)
			default:
				c.write("Unknown command.\n")
			}
		case stateDead:
			break
		}
	}
	c.conn.Close()
	delete(m.conns, c.conn.RemoteAddr().String())
	if c.state == statePlaying {
		m.broadcast(c.name + " has left the game.\n")
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
	for _, c := range m.conns {
		if c.state == statePlaying {
			c.write("  " + c.name + "\n")
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
