package network

import (
    "encoding/gob"
    "fmt"
    "log"
    "net"
    "sync"
    "io"
)

type Pool struct {
    connections map[net.Conn]struct{}
    size        int
    mx          sync.Mutex
}

func (p *Pool) Add(conn net.Conn) error {
    p.mx.Lock()
    defer p.mx.Unlock()

    remoteAddr := conn.RemoteAddr().String()

    if _, ok := p.connections[conn]; ok {
        return fmt.Errorf("Connection already exists", remoteAddr)
    }

    if len(p.connections) >= p.size {
        return fmt.Errorf("Connection pool is full", remoteAddr)
    }

    p.connections[conn] = struct{}{}

    log.Println("Added new connection", remoteAddr)

    return nil
}

func (p *Pool) Remove(conn net.Conn) {
    p.mx.Lock()
    defer p.mx.Unlock()

    if _, ok := p.connections[conn]; ok {
        delete(p.connections, conn)

        if err := conn.Close(); err != nil {
            log.Println(err)
        } else {
            log.Println("Connection closed", conn.RemoteAddr())
        }
    }
}

func (p *Pool) Broadcast(msg Message) {
    p.mx.Lock()
    defer p.mx.Unlock()

    for conn, _ := range p.connections {
        encoder := gob.NewEncoder(conn)

        if err := encoder.Encode(msg); err != nil {
            log.Println(err)
            p.Remove(conn)
        }

		    log.Println("Broadcasted message", msg.Kind, "to:", conn.RemoteAddr())
    }
}

func (p *Pool) ListConnections() []net.Conn {
    p.mx.Lock()
    defer p.mx.Unlock()

    connections := make([]net.Conn, 0, len(p.connections))

    for conn, _ := range p.connections {
        connections = append(connections, conn)
    }

    return connections
}

func NewPool(peers []string, size int) *Pool {
    pool := &Pool{
        connections: make(map[net.Conn]struct{}),
        size: size,
    }

    for _, peer := range peers {
        conn, err := net.Dial("tcp", peer)

        if err != nil {
            log.Println(err)
            continue
        }

        if err = pool.Add(conn); err != nil {
            log.Println(err)
        }
    }

    return pool
}

func HandleConnection(pool *Pool, conn net.Conn, callback func(message string)) {
    decoder := gob.NewDecoder(conn)

    for {
        var msg Message

        if err := decoder.Decode(&msg); err != nil {
            if err == io.EOF {
                log.Println("Connection closed", conn.RemoteAddr())
            } else {
                log.Println("Failed to decode message from", conn.RemoteAddr(), " | ", err)
            }

            pool.Remove(conn)
            return
        }

        callback(msg.Kind)
        pool.Broadcast(msg)
    }
}

