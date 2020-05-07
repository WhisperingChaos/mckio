/*
Package mckio offers mock/simulated readers for certain io devices whose rigid OS coupling complicates testing.
*/
package mckio

import (
	"io"
	"time"
)

/*
Rstrings implements an io.reader provided a list of strings that are reflected
and consumed by the Read method as a series of bytes.

The following behavior of Rstrings can be configured:

- BehaviorDelimer (optional) - specifies delimiters that are
concatenated to the end of each string comprising the list of strings.
When undefined - no concatenation occurs.

- BehaviorBlockAtEnder (optional) - specifies an implementation that
blocks the reader after the list of strings has been exhausted instead
of signaling io.EOF.  When undefined - signals io.EOF.

- BehaviorBlockBeforeEachReader (optional) - specifies an implementation
blocking the reader before it attempts to read the first/next string.
When undefined - the read immediately executes.

Notes

- Although golang defines a string as "just a bunch of bytes" use caution
because it may contain different encodings that might not be
compatible to the component consuming the bytes returned by io.Read
(https://blog.golang.org/strings).

- Rstrings implementation is not concurrency safe.
*/
type Rstrings struct {
	lcur        int
	ccur        int
	dcur        int
	list        []string
	delim       []byte
	blockBefore func()
	block       func()
}

/*
BehaviorDelimer defines one or more byte values as a delimiter concatenated
to each element of a list of strings.
*/
type BehaviorDelimer interface {
	BehaviorDelim() []byte
}

/*
BehaviorBlockAtEnder provides a blocking mechanism that's executed once
the reader has been exhausted.  A select{} statement offers a simple
implementation that forever blocks.
*/
type BehaviorBlockAtEnder interface {
	BehaviorBlockAtEnd()
}

/*
BehaviorBlockBeforeEachReader provides a blocking mechanism that's executed
at the start of every read.
*/
type BehaviorBlockBeforeEachReader interface {
	BehaviorBlockBeforeEachRead()
}

/*
NewRstrings implements an io.Reader interface over a list of strings.  Its
behavior can be configured to:

- optionally block at the start of each read call,

- optionally concatenate a delimiter sequence at the end of each string element,

- optionally block after the entire list of strings has been exhausted.

Independently specify these behaviors using BehaviorBlockBeforeEachReader,
BehaviorDelimer, and BehaviorBlockAtEnder.
*/
func NewRstrings(list []string, behavior interface{}) (rdr Rstrings) {
	rdr.list = list
	if pd, ok := behavior.(BehaviorDelimer); ok {
		rdr.delim = pd.BehaviorDelim()
	}
	rdr.block = func() {}
	if bk, ok := behavior.(BehaviorBlockAtEnder); ok {
		rdr.block = func() {
			bk.BehaviorBlockAtEnd()
		}
	}
	rdr.blockBefore = func() {}
	if bkb, ok := behavior.(BehaviorBlockBeforeEachReader); ok {
		rdr.blockBefore = func() {
			bkb.BehaviorBlockBeforeEachRead()
		}
	}
	return rdr
}

/*
Read implements an io.Reader based on a slice of strings conforming to
io.Reader semantics (https://golang.org/pkg/io/#Reader).
*/
func (m *Rstrings) Read(p []byte) (int, error) {
	if len(p) == 0 {
		// if blocking before read want to return before blocking
		// when requesting 0 bytes - do nothing.
		return 0, nil
	}
	m.blockBefore()
	var pi int
	for ; m.lcur < len(m.list); m.lcur++ {
		for ; m.ccur < len(m.list[m.lcur]); m.ccur++ {
			if pi < len(p) {
				p[pi] = ([]byte(m.list[m.lcur]))[m.ccur]
				pi++
			} else {
				return len(p), nil
			}
		}
		for ; m.dcur < len(m.delim); m.dcur++ {
			if pi < len(p) {
				p[pi] = m.delim[m.dcur]
				pi++
			} else {
				return len(p), nil
			}
		}
		m.dcur = 0
		m.ccur = 0
	}
	if pi < 1 {
		m.block()
		// if block Behavior doesn't block then return EOF
		return 0, io.EOF
	}
	return pi, nil
}

/*
NewConsole similulates an io.Reader on os.Stdin.  It implements this
simulation by composing:

- BehaviorDelim - newline deliminter at the end of every string element,

- BehaviorBlockBeforeEachRead - blocks 1 second before allowing read, and

- BehaviorBlockAtEnd - executing a block after exhausting the list of provided strings.
*/
func NewConsole(cmdLns []string) (rdr Rstrings) {
	return NewRstrings(cmdLns, stdin{})
}

/*
NewNonBlockNoDelim simply streams a list of provided strings for reading
without blocking nor adding any type of delimiter at each string's end.  It
implements the default behavior of Rstrings.
*/
func NewNonBlockNoDelim(cmdLns []string) (rdr Rstrings) {
	return NewRstrings(cmdLns, nil)
}

/*
Rchan converts a channel streaming strings into an io.Reader.

- This reader can block because the channel can block.

- The reader will return as many residual bytes from the previous
read before requesting data from the channel.  Therefore, be prepared to
receive a quantity of bytes less than the length retuested by the
'p []byte' argument.

- Closing the channel returns an io.EOF error.

Note

- Although golang defines a string as "just a bunch of bytes" use caution
because it may contain different encodings that might not be
compatible to the component consuming the bytes returned by io.Read
(https://blog.golang.org/strings).


- Rchan is not concurrency safe.
*/
type Rchan struct {
	cmdLn <-chan string
	sCur  string
	spos  int
}

/*
NewChan creates an io.Reader implemented as a receiving channel of strings.
*/
func NewChan(cmdLn <-chan string) (rdr Rchan) {
	return Rchan{cmdLn: cmdLn}
}

/*
Read implements an io.Reader based on a channel conforming to
io.Reader semantics (https://golang.org/pkg/io/#Reader).
*/
func (rc *Rchan) Read(p []byte) (int, error) {
	if len(p) == 0 {
		// because channel can block - return do nothing request instead
		// of blocking and then returning nothing.
		return 0, nil
	}
	var ip int
	for {
		for ; rc.spos < len(rc.sCur) && ip < len(p); rc.spos, ip = rc.spos+1, ip+1 {
			p[ip] = ([]byte(rc.sCur))[rc.spos]
		}
		if ip > 0 {
			// have something to return.  do so before
			// possibly blocking on channel.
			return ip, nil
		}
		var ok bool
		rc.sCur, ok = <-rc.cmdLn
		if !ok {
			return 0, io.EOF
		}
		rc.spos = 0
	}
}

//-----------------------------------------------------------------------------
//--                         Private Section                                ---
//-----------------------------------------------------------------------------

type stdin struct{}

func (stdin) BehaviorDelim() (delim []byte) {
	delim = []byte{'\n'}
	return delim
}
func (stdin) BehaviorBlockAtEnd() {
	select {}
}
func (stdin) BehaviorBlockBeforeEachRead() {
	time.Sleep(1 * time.Second)
}
