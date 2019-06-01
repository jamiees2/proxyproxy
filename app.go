package main

import (
    proxyproto "github.com/pires/go-proxyproto"
    "log"
    "net"
    "os"
    "strconv"
)

// Server is a TCP server that takes an incoming request and sends it to another
// server, proxying the response back to the client.
type Server struct {
    // TCP address to listen on
    Addr string

    // TCP address of target server
    Target string

    // ModifyRequest is an optional function that modifies the request from a client to the target server.
    ModifyRequest func(b *[]byte)

    // ModifyResponse is an optional function that modifies the response from the target server.
    ModifyResponse func(b *[]byte)

    AddHeaders func(conn net.Conn)
}

// ListenAndServe listens on the TCP network address laddr and then handle packets
// on incoming connections.
func (s *Server) ListenAndServe() error {
    log.Printf("Listening on %s\n", s.Addr)
    listener, err := net.Listen("tcp", s.Addr)
    if err != nil {
        return err
    }
    return s.serve(listener)
}

func (s *Server) serve(ln net.Listener) error {
    for {
        conn, err := ln.Accept()
        if err != nil {
            log.Println(err)
            continue
        }
        go s.handleConn(conn)
    }
}

func (s *Server) handleConn(conn net.Conn) {
    // connects to target server
    rconn, err := net.Dial("tcp", s.Target)
    if err != nil {
        log.Printf("Error while establishing connection to upstream: %v\n", err)
        return
    }

    if s.AddHeaders != nil {
        s.AddHeaders(rconn)
    }

    // write to dst what it reads from src
    var pipe = func(src, dst net.Conn, filter func(b *[]byte)) {
        defer func() {
            conn.Close()
            rconn.Close()
        }()

        buff := make([]byte, 65535)
        for {
            n, err := src.Read(buff)
            if err != nil {
                log.Println(err)
                return
            }
            b := buff[:n]
            if filter != nil {
                filter(&b)
            }
            _, err = dst.Write(b)
            if err != nil {
                log.Println(err)
                return
            }
        }
    }

    go pipe(conn, rconn, s.ModifyRequest)
    go pipe(rconn, conn, s.ModifyResponse)

}
func getEnv(key, fallback string) string {
    if value, ok := os.LookupEnv(key); ok {
        return value
    }
    return fallback
}

var (
    localAddr  = getEnv("LISTENER_HOST", ":4444")
    targetAddr = getEnv("DEST_HOST", ":80")
    sourceAddr = getEnv("SOURCE_ADDR", "127.0.0.1")
    sourcePort, _ = strconv.Atoi(getEnv("SOURCE_PORT", "0"))
    destAddr = getEnv("DEST_ADDR", "127.0.0.1")
    destPort, _ = strconv.Atoi(getEnv("DEST_PORT", "0"))
)



func main() {
    proxyheader := proxyproto.Header  {
        Version: 2,
        Command: proxyproto.PROXY,
        TransportProtocol: proxyproto.TCPv4,
        SourceAddress: net.ParseIP(sourceAddr),
        DestinationAddress: net.ParseIP(destAddr),
        SourcePort: uint16(sourcePort),
        DestinationPort: uint16(destPort),
    }
    addHeaders := func(dest net.Conn) {
        _, err := proxyheader.WriteTo(dest)
        if err != nil {
            log.Println(err)
            dest.Close()
        }
    }
    p := Server{
        Addr:   localAddr,
        Target: targetAddr,
        AddHeaders: addHeaders,
    }

    err := p.ListenAndServe()
    if err != nil {
        log.Fatal(err)
    }

}
