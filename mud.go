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
	mud    *mud
}

// mud represents the MUD server.
type mud struct {
	listener net.Listener
	conns    map[string]*connection
    rooms    map[string]*room
}

// positionHash returns a hash of the given x and y position.
func positionHash(x, y int) string {
    return fmt.Sprintf("%04d%04d", x, y)
}

// room represents a room in the MUD.
type room struct {
    name        string
    description string
    x           int
    y           int
    exits       map[string]string
}

// newRoom creates a new room.
func newRoom(name, description string) *room {
    return &room{
        name:        name,
        description: description,
        exits:       make(map[string]string),
    }
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
        listener: nil,
        conns:    make(map[string]*connection),
        rooms:    make(map[string]*room),
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
    case "look":
        m.look(c)
	case "who":
		m.who(c)
	case "say":
		m.say(c, args)
	case "quit":
		m.quit(c)
    case "north":
        m.move(c, "north")
    case "east":
        m.move(c, "east")
    case "south":
        m.move(c, "south")
    case "west":
        m.move(c, "west")
	default:
		c.write("Unknown command.\n")
	}
	if c.state == statePlaying {
		c.write(fmt.Sprintf("%s: %d/%d > ", c.name, c.player.health, c.player.mana))
	}
}

// move moves the player in the given direction if an exit exists in that direction.
func (m *mud) move(c *connection, dir string) {
    p := c.player

    // check if an exit exists in the given direction
    exitHash, ok := m.getRoomByPosition(p.x, p.y).exits[dir]
    if !ok {
        // no exit exists in the given direction, so do nothing
        c.write("You cannot go that way.\n")
        return
    }

    // move the player to the room in the given direction
    p.x, p.y = m.getRoomPositionFromHash(exitHash)
    c.write(fmt.Sprintf("You move %s.\n", dir))
    m.look(c)
}

// getRoomByPosition returns the room at the given position.
func (m *mud) getRoomByPosition(x, y int) *room {
    return m.rooms[positionHash(x, y)]
}

// getRoomPositionFromHash returns the position of the room with the given position hash.
func (m *mud) getRoomPositionFromHash(hash string) (int, int) {
    x, y := 0, 0
    fmt.Sscanf(hash, "%04d%04d", &x, &y)
    return x, y
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

// handleLook processes the look command for the given connection.
func (m *mud) look(c *connection) {
    // get the player's current position
    x, y := c.player.x, c.player.y

    // look up the room at the player's position
    key := positionHash(x, y)
    r, ok := m.rooms[key]
    if !ok {
        c.write("You are lost in the void.\n")
        return
    }

    // write the room name and description
    c.write(fmt.Sprintf("%s\n", r.name))
    c.write(fmt.Sprintf("%s\n", r.description))

    // write the exits from the room
    c.write("Exits:\n")
    for dir, key := range r.exits {
        r2, ok := m.rooms[key]
        if !ok {
            continue
        }
        c.write(fmt.Sprintf("%s - %s\n", dir, r2.name))
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

// addRoom adds a room at the given position.
func (m *mud) addRoom(x, y int, r *room) {
    r.x, r.y = x, y
    m.rooms[positionHash(x, y)] = r
}

// addExit adds an exit in the given direction from the given room to another room.
func (m *mud) addExit(r *room, dir string) {
    // check if a room already exists in the given direction
    x, y := r.x, r.y
    switch dir {
    case "north":
        y++
    case "east":
        x++
    case "south":
        y--
    case "west":
        x--
    default:
        return
    }
    key := positionHash(x, y)
    r2, ok := m.rooms[key]
    if !ok {
        // no room exists in the given direction, so do nothing
        return
    }

    // add the exit from the given room to the room in the given direction
    r.exits[dir] = key

    // add the return exit from the room in the given direction to the given room
    var returnDir string
    switch dir {
    case "north":
        returnDir = "south"
    case "east":
        returnDir = "west"
    case "south":
        returnDir = "north"
    case "west":
        returnDir = "east"
    }
    r2.exits[returnDir] = positionHash(r.x, r.y)
}

// getExit looks up the room in the given direction.
func (r *room) getExit(direction string, rooms map[string]*room) *room {
    key, ok := r.exits[direction]
    if !ok {
        return nil
    }
    return rooms[key]
}

// write sends the given message to the connection.
func (c *connection) write(msg string) {
	c.output.WriteString(msg)
	c.output.Flush()
}

// center returns the given string padded with spaces so that it is centered
// within the given width.
func center(s string, width int) string {
    pad := width - len(s)
    if pad < 0 {
        return s
    }
    left := pad / 2
    right := pad - left
    return strings.Repeat(" ", left) + s + strings.Repeat(" ", right)
}

// createMap creates the map of rooms and adds them to the given mud struct.
func (m *mud)createMap() {
    // create rooms
	r1 := newRoom("Mall Entrance", "The mall entrance is bustling with people coming and going.")
	r2 := newRoom("Directory", "The directory is a large board listing all the stores in the mall.")
	r3 := newRoom("Food Court", "The food court is full of the smells and sounds of various restaurants.")
	r4 := newRoom("Arcade", "The arcade is filled with flashing lights and the sounds of games.")
	r5 := newRoom("Restroom", "The restroom is clean and well-maintained.")
	r6 := newRoom("Toy Store", "The toy store is filled with rows of colorful toys and games.")
	r7 := newRoom("Electronics Store", "The electronics store is full of the latest gadgets and technology.")
	r8 := newRoom("Clothing Store", "The clothing store is full of racks of clothes in the latest styles.")
	r9 := newRoom("Shoe Store", "The shoe store is filled with rows of shoes of all shapes, sizes, and colors.")
	r10 := newRoom("Sporting Goods Store", "The sporting goods store is full of a wide variety of sports equipment and apparel.")

    // add rooms to the map
    m.rooms[positionHash(0, 0)] = r1
    m.rooms[positionHash(1, 0)] = r2
    m.rooms[positionHash(2, 0)] = r3
    m.rooms[positionHash(3, 0)] = r4
    m.rooms[positionHash(3, 1)] = r5
    m.rooms[positionHash(3, 2)] = r6
    m.rooms[positionHash(2, 2)] = r7
    m.rooms[positionHash(1, 2)] = r8
    m.rooms[positionHash(0, 2)] = r9
    m.rooms[positionHash(0, 1)] = r10

	// add exits
	m.addExit(r1, "north")
	m.addExit(r1, "south")
	m.addExit(r2, "west")
	m.addExit(r2, "east")
	m.addExit(r3, "north")
	m.addExit(r3, "south")
	m.addExit(r3, "west")
	m.addExit(r4, "west")
	m.addExit(r4, "east")
	m.addExit(r5, "south")
	m.addExit(r5, "east")
	m.addExit(r6, "west")
	m.addExit(r6, "north")
	m.addExit(r7, "west")
	m.addExit(r7, "south")
	m.addExit(r8, "west")
	m.addExit(r8, "north")
	m.addExit(r9, "west")
	m.addExit(r9, "north")
	m.addExit(r10, "west")
	m.addExit(r10, "north")
}


func main() {
	m := newMud()

	m.createMap()

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
