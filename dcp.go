package dcp

import (
	"crypto/md5"
	"encoding/gob"
	"fmt"
	"io/ioutil"
	"net"
	"os"
	"path"
	"strings"
)

func init() {
	gob.Register(&FileChunk{})
}

// Verbose to true to enable output
var Verbose bool

func verb(format string, args ...interface{}) {
	if Verbose {
		fmt.Printf(format, args...)
	}
}

// File contains needed metadata to compare and copy file accross the world
type File struct {
	Name string
	Sum  [md5.Size]byte
	Path string
	Size int64
}

// Nice constants and stuff
const (
	BufferSize = 4 * 1024
	DIFF       = "DIFF"
	GET        = "GET"
	PUT        = "PUT"
	REMOVE     = "REMOVE"
	FILE       = "FILE"
)

// Order defines an action to be done either on client on server
type Order struct {
	Type  string
	Tag   string
	Files []File
}

// Diff contains all differences betwen two folders tree
type Diff struct {
	ClientNew []File
	ServerNew []File
	Modified  []File
}

// FileChunk is a gob encoded struct containing file chunk
type FileChunk struct {
	ID   int
	Data []byte
	Size int
}

// Sum returns the md5sum of the file
func Sum(filepath string) ([md5.Size]byte, error) {
	var sum [md5.Size]byte

	data, err := ioutil.ReadFile(filepath)
	if err != nil {
		return sum, err
	}

	sum = md5.Sum(data)
	return sum, nil
}

// List a directory recursively
func List(directory string) ([]File, error) {
	var files []File

	fis, err := ioutil.ReadDir(directory)
	if err != nil {
		return nil, err
	}

	for _, fi := range fis {
		if fi.IsDir() {
			subfiles, err := List(path.Join(directory, fi.Name()))
			if err != nil {
				return nil, err
			}
			files = append(files, subfiles...)
		} else {

			sum, err := Sum(path.Join(directory, fi.Name()))
			if err != nil {
				return nil, err
			}

			f := File{
				Name: fi.Name(),
				Sum:  sum,
				Path: path.Join(directory, fi.Name()),
				Size: fi.Size(),
			}
			files = append(files, f)
		}
	}

	return files, nil
}

// GetDiff connects to given host and get the differences between host and local directory
func GetDiff(endpoint string, directory string, tag string) (*Diff, error) {

	conn, err := net.Dial("tcp", endpoint)
	if err != nil {
		return nil, err
	}

	files, err := List(directory)
	if err != nil {
		return nil, err
	}

	err = sendOrder(conn, DIFF, tag, files)
	if err != nil {
		return nil, err
	}

	diff, err := receiveDiff(conn)
	if err != nil {
		return nil, err
	}
	return diff, nil
}

// CopyTo transfers files from given directory into host at given endpoint in folder tag.
func CopyTo(endpoint string, directory string, tag string) error {

	conn, err := net.Dial("tcp", endpoint)
	if err != nil {
		return err
	}
	defer conn.Close()

	files, err := List(directory)
	if err != nil {
		return err
	}

	err = sendOrder(conn, DIFF, tag, files)
	if err != nil {
		return err
	}

	diff, err := receiveDiff(conn)
	if err != nil {
		return err
	}

	// Removing files present only on host
	if len(diff.ServerNew) > 0 {
		err = sendOrder(conn, REMOVE, tag, diff.ServerNew)
		if err != nil {
			return err
		}
		for _, f := range diff.ServerNew {
			verb("REMOVE %s\n", f.Path)
		}
	}

	// "Update" files modified
	err = sendOrder(conn, REMOVE, tag, diff.Modified)
	if err != nil {
		return err
	}
	for _, f := range diff.Modified {
		verb("REMOVE %s\n", f.Path)
		verb("PUSH   %s\n", f.Path)
		notOK := true
		for notOK {
			err := SendFile(conn, directory, f, tag)
			if err == nil {
				notOK = false
			} else {
				conn, err = net.Dial("tcp", endpoint)
				if err != nil {
					return err
				}
			}
		}
	}

	for _, f := range diff.ClientNew {
		verb("PUSH    %s (%x)...\n", f.Name, f.Sum)
		notOK := true
		for notOK {
			err := SendFile(conn, directory, f, tag)
			if err == nil {
				notOK = false
			} else {
				conn, err = net.Dial("tcp", endpoint)
				if err != nil {
					return err
				}
			}
		}
	}

	return nil
}

func sendOrder(conn net.Conn, command string, tag string, files []File) error {
	o := Order{
		Type:  command,
		Tag:   tag,
		Files: files,
	}

	enc := gob.NewEncoder(conn)
	return enc.Encode(o)
}

func receiveOrder(conn net.Conn) (*Order, error) {
	order := &Order{}
	dec := gob.NewDecoder(conn)
	err := dec.Decode(order)
	if err != nil {
		return nil, err
	}
	return order, nil
}

func receiveDiff(conn net.Conn) (*Diff, error) {
	diff := &Diff{}
	dec := gob.NewDecoder(conn)
	err := dec.Decode(diff)
	if err != nil {
		return nil, err
	}

	return diff, nil
}

// CopyFrom transfers all file from host in tag into directory
func CopyFrom(endpoint string, tag string, directory string) error {
	return nil
}

// SendFile transfers the given file onto host trough given conn.
func SendFile(conn net.Conn, basedir string, file File, tag string) error {
	p := path.Join(basedir, file.Path)
	bs := BufferSize

	data, err := ioutil.ReadFile(p)
	if err != nil {
		return err
	}

	// Send ordre to remote
	err = sendOrder(conn, FILE, tag, []File{file})
	if err != nil {
		return err
	}

	// Split file in BufferSize parts
	chunks := len(data) / bs
	if len(data)%bs != 0 {
		chunks++
	}
	enc := gob.NewEncoder(conn)

	for i := 0; i < chunks; i++ {
		a := i * bs
		if a >= len(data) {
			continue
		}
		b := (i + 1) * bs
		if b > len(data) {
			b = len(data)
		}
		if b < 0 {
			b = 0
		}
		fc := FileChunk{
			ID:   i + 1,
			Size: b - a,
			Data: data[a:b],
		}

		err = enc.Encode(fc)
		if err != nil {
			return err
		}
	}

	fc := FileChunk{Size: -1}
	err = enc.Encode(fc)
	if err != nil {
		return err
	}

	// Wait for ack
	f := &File{}
	dec := gob.NewDecoder(conn)
	err = dec.Decode(f)
	if err != nil {
		return err
	}
	return nil
}

// ReceiveFile read gob encoded file chunks and write them on given tag
func ReceiveFile(conn net.Conn, basedir string, tag string, file File) error {
	verb("ReceiveFile %s/%s\n", tag, file.Path)
	p := path.Join(basedir, tag, file.Path)
	dir := strings.TrimSuffix(p, file.Name)

	os.MkdirAll(dir, 0755)

	f, err := os.OpenFile(p, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0600)
	if err != nil {
		return fmt.Errorf("Cannot open file: %s\n", err)
	}
	defer f.Close()

	dec := gob.NewDecoder(conn)
	for {
		// read
		var chunk FileChunk
		err := dec.Decode(&chunk)
		if err != nil {
			return fmt.Errorf("%s: Cannot decode file chunk: %s\n", file.Name, err)
		}
		if chunk.Size == 0 {
			fmt.Printf("%s: Skip (0)\n", file.Name)
			continue
		}
		if chunk.Size < 0 {
			verb("%s: Done (<0)\n", file.Name)
			break
		}
		if len(chunk.Data) != chunk.Size {
			return fmt.Errorf("Received %d bytes, wanted %d\n", len(chunk.Data), chunk.Size)
		}

		// and write
		if _, err = f.Write(chunk.Data); err != nil {
			return fmt.Errorf("cannot write to file: %s\n", err)
		}
	}
	enc := gob.NewEncoder(conn)
	return enc.Encode(file)
}

func diff(serverFiles []File, clientFiles []File) *Diff {
	d := &Diff{}

	// searching for file existing but non identical
	for _, s := range serverFiles {
		for _, c := range clientFiles {
			if s.Name == c.Name && s.Path == c.Path && s.Sum != c.Sum {
				d.Modified = append(d.Modified, s)
			}
		}
	}

	// a client file is a file presend on client but not on server
	for _, c := range clientFiles {
		// Check if it's there remotely
		found := false
		for _, s := range serverFiles {
			if s.Path == c.Path && s.Name == c.Name {
				found = true
			}
		}
		if !found {
			d.ClientNew = append(d.ClientNew, c)
		}
	}

	// a created file is a file present on server but not on client side
	for _, s := range serverFiles {
		found := false
		for _, c := range clientFiles {
			if s.Name == c.Name && s.Path == c.Path {
				found = true
			}
		}
		if !found {
			d.ServerNew = append(d.ServerNew, s)
		}
	}

	return d
}
