package dcp

import (
	"encoding/gob"
	"fmt"
	"io"
	"net"
	"os"
	"path"
	"strings"
)

// Host starts listening on given address and manipulation file within given base directory.
func Host(addr string, basedir string) error {
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return err
	}

	for {
		conn, err := ln.Accept()
		if err != nil {
			if Verbose {
				fmt.Printf("Cannot accept new connection: %s\n", err)
			}
			continue
		}
		go dealWithIt(conn, basedir)
	}
}

func dealWithIt(conn net.Conn, basedir string) {

	for {
		o, err := receiveOrder(conn)
		if err == io.EOF {
			return // We are DONE HERE FFS
		}
		if err != nil {
			fmt.Printf("%v: Cannot receive order: %s\n", conn, err)
			return
		}

		switch o.Type {
		case DIFF:
			dealWithDiff(conn, basedir, o)
			break
		case PUT:
			break
		case GET:
			break
		case REMOVE:
			err = dealWithRemove(conn, basedir, o)
			break
		case FILE:
			err = dealWithFile(conn, basedir, o)
			break
		}

		if err != nil {
			verb("%s: %s\n", o.Type, err)
			conn.Close()
			return
		}
	}
}

func dealWithRemove(conn net.Conn, basedir string, o *Order) error {

	if o.Tag == "" {
		return fmt.Errorf("Invalid tag. aborting\n")
	}

	for _, f := range o.Files {
		verb("Removing %s/%s\n", o.Tag, f.Path)
		p := path.Join(basedir, o.Tag, f.Path)
		err := os.Remove(p)
		if err != nil {
			return fmt.Errorf("Cannot remove file %s %s\n", o.Tag, f.Path)
		}
	}

	return nil
}

func dealWithFile(conn net.Conn, basedir string, o *Order) error {
	verb("File on %s/%s\n", o.Files[0].Path, o.Files[0].Path)
	return ReceiveFile(conn, basedir, o.Tag, o.Files[0])
}

func dealWithDiff(conn net.Conn, basedir string, o *Order) {
	verb("Diff on %s\n", o.Tag)

	p := path.Join(basedir, o.Tag)
	files, err := List(p)
	if err != nil {
		fmt.Printf("Cannot list tag %s (%s)\n", o.Tag, p)
	}
	for i := range files {
		files[i].Path = strings.TrimPrefix(files[i].Path, o.Tag+"/")
	}

	d := diff(files, o.Files)

	enc := gob.NewEncoder(conn)
	err = enc.Encode(d)
	if err != nil {
		fmt.Printf("Cannot send diff to client: %s\n", err)
		return
	}
}
