package mckio

import (
	"io"
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_RchanMessageSimple(t *testing.T) {
	assrt := assert.New(t)
	cmdLn := make(chan string)
	defer close(cmdLn)
	rdr := NewChan(cmdLn)
	msg := "Rchan begin"
	go func() { cmdLn <- msg }()
	buf := make([]byte, len(msg))
	sz, err := rdr.Read(buf)
	assrt.Equal(len(msg), sz)
	assrt.Nil(err)
	assrt.Equal([]byte(msg), buf)
}
func Test_RchanReadDoNothing(t *testing.T) {
	assrt := assert.New(t)
	var cmdLn chan string
	rdr := NewChan(cmdLn)
	var buf []byte
	sz, err := rdr.Read(buf)
	assrt.Zero(sz)
	assrt.Nil(err)
}

func Test_RchanSegmentAcrossCalls(t *testing.T) {
	assrt := assert.New(t)
	cmdLn := make(chan string)
	rdr := NewChan(cmdLn)
	msg := "0123456789"
	go func() { cmdLn <- msg }()
	buf := make([]byte, len(msg)/2+1)
	sz, err := rdr.Read(buf)
	assrt.Equal(len(msg)/2+1, sz)
	assrt.Nil(err)
	assrt.Equal([]byte(msg)[0:len(msg)/2+1], buf)
	// read should not block as it prefers to return
	// remnant bytes from prior call before
	sz, err = rdr.Read(buf)
	assrt.Equal(len(msg)-(len(msg)/2+1), sz)
	assrt.Nil(err)
	assrt.Equal([]byte(msg)[len(msg)/2+1:], buf[0:sz])
	// read shoud generate io.EOF
	close(cmdLn)
	sz, err = rdr.Read(buf)
	assrt.Equal(0, sz)
	assrt.IsType(io.EOF, err)
	// read after io.EOF should generate io.EOF
	sz, err = rdr.Read(buf)
	assrt.Equal(0, sz)
	assrt.IsType(io.EOF, err)
}

func Test_RchanReadByteCntSmallerThanBuffer(t *testing.T) {
	assrt := assert.New(t)
	cmdLn := make(chan string)
	rdr := NewChan(cmdLn)
	msg := "0123456789"
	msgNum := 10
	go func() {
		defer close(cmdLn)
		for i := 0; i < msgNum; i++ {
			cmdLn <- msg
		}
	}()
	buf := make([]byte, len(msg)*2+1)
	for i := 0; i < msgNum; i++ {
		// should complete all reads without blocking
		sz, err := rdr.Read(buf)
		assrt.Equal(len(msg), sz)
		assrt.Nil(err)
		assrt.Equal([]byte(msg), buf[0:len(msg)])
	}
	// read should not block
	sz, err := rdr.Read(buf)
	assrt.Equal(0, sz)
	assrt.IsType(io.EOF, err)
}
func Test_RstringsReadDoNothing(t *testing.T) {
	assrt := assert.New(t)
	var cmds []string
	rdr := NewNonBlockNoDelim(cmds)
	var p []byte
	sz, err := rdr.Read(p)
	assrt.Zero(sz)
	assrt.Nil(err)
}

func Test_RstringsConsumeAllAtOnce(t *testing.T) {
	assrt := assert.New(t)
	cmds := []string{"cmmd 1", "cmmd 2", "cmmd 3"}
	rdr := NewNonBlockNoDelim(cmds)
	p := make([]byte, byteSizeCalc(cmds))
	sz, err := rdr.Read(p)
	assrt.Equal(byteSizeCalc(cmds), sz)
	assrt.Nil(err)
	// above should have consumed the entire list of strings
	// check for EOF on next call.
	sz, err = rdr.Read(p)
	assrt.Zero(sz)
	assrt.IsType(io.EOF, err)
}

func Test_RstringsConsume1AtaTime(t *testing.T) {
	assrt := assert.New(t)
	cmds := []string{"cmmd 1", "cmmd 2", "cmmd 3"}
	rdr := NewNonBlockNoDelim(cmds)
	// assumes strings are all of equal length
	p := make([]byte, (byteSizeCalc(cmds)+1)/3)
	for i := 0; i < len(cmds); i++ {
		sz, err := rdr.Read(p)
		assrt.Equal(len(cmds[i]), sz)
		assrt.Equal([]byte(cmds[i]), p)
		assrt.Nil(err)
	}
	// above should have consumed the entire list of strings
	// check for EOF on next call.
	sz, err := rdr.Read(p)
	assrt.Zero(sz)
	assrt.IsType(io.EOF, err)
}
func byteSizeCalc(list []string) (sizeTotal int) {
	for _, s := range list {
		sizeTotal += len(s)
	}
	return sizeTotal
}
func Test_RstringConsole(t *testing.T) {
	assrt := assert.New(t)
	cmds := []string{"cmmd 1", "cmmd 2", "cmmd 3"}
	rdr := NewConsole(cmds)
	// assumes strings are all of equal length
	// added newline delimiter at the end of each
	p := make([]byte, (byteSizeCalc(cmds)+len(cmds)*1+1)/len(cmds))
	for i := 0; i < len(cmds); i++ {
		sz, err := rdr.Read(p)
		assrt.Equal(len(cmds[i])+1, sz)
		assrt.Equal([]byte(cmds[i]), p[0:sz-1])
		assrt.Nil(err)
	}
	// above should have consumed the entire list of strings
	// another read should cause block.
	// rdr.Read(p)
}
func Test_RstringReadBufIntergralOfDelimAndStringSize(t *testing.T) {
	assrt := assert.New(t)
	cmds := []string{"cmmd 1", "cmmd 2", "cmmd 3"}
	rdr := NewRstrings(cmds, delimAdd{})
	// assumes strings are all of equal length
	// added newline delimiter at the end of each
	p := make([]byte, len(cmds[0]))
	var rslt string
	for i := 0; i < len(cmds)+1; i++ {
		sz, err := rdr.Read(p)
		rslt += string(p[0:sz])
		assrt.Nil(err)
	}
	var expct string
	for _, cmd := range cmds {
		expct += (cmd + string(delimAdd{}.BehaviorDelim()))
	}
	assrt.Equal(expct, rslt)
	// above should have consumed the entire list of strings
	// another read should cause block.
	// rdr.Read(p)
}

type delimAdd struct {
}

func (delimAdd) BehaviorDelim() (delims []byte) {
	return []byte{'\n'}
}
